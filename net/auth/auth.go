/*
	Authentication services for clive.

	Clive relies on challenge response authentication using shared keys.
	This package provides tools to authenticate clients and servers and
	to authenticate acess to wax interfaces.

	auth.Conn is the primary interface.
	All other tools are helpers (and there are even more helpers not
	exported in case you need an auth related function; just look at the code).
*/
package auth

// BUG(x): the secrets file for a domain should keep multiple user/key pairs.

// REFERENCE(x): nchan, channels for I/O devices.

// REFERENCE(x): cmd/auth, to generate key files.

import (
	"bufio"
	"bytes"
	"clive/ch"
	"clive/dbg"
	"clive/u"
	"clive/x/code.google.com/p/go.crypto/pbkdf2"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"
)

/*
	Auth info resulting from authentication.

	This is most often used to perform file access permission checking.
	The expected semantics are as follow, as a reference for file server
	implementors:

	- A remote user is defined by the Info structure, including the set of groups
	it belongs to. Locally, the user uid and its groups are defined by a similar
	Key structure.

	- A file server relies on this package (clive/net/auth) to authenticate a user,
	and define the list of (locally defined groups) it belongs to.

	- Both the user and the group are local concepts, and authentication is
	used to attach a remote user to a locally defined user.

	- If there is no auth info the file server usually grants access (no-auth).

	- If the user is the file owner, and any of the u/g/o permissions are set,
	then permission is granted.

	- If the user has a gid listed that is that of the file, and any of the g/o
	permissions are set, permission is granted.

	- Only members of sys may change the owner of a file.

	- The group of a file may be changed by users in that group or by users
	in the sys group.

	- User defined attributes may be changed only if the user can write
	the file, or is the owner of the file.

	- The mode may be changed by the file owner.

	- Files (re)created in a directory are created with the group of that
	directory, and permissions are filtered according to those of the directory.

	- When files are being created, if Uid/Gid are specified and the rules
	described would not grant permission, such attributes are removed from
	the request and no error is produced because of them. This is so common
	due to wstats carrying uids/gids that reporting an error would cause more
	harm than not doing so in this case.

	- Otherwise permission checking is similar to that of unix (v6).
*/
struct Info {
	Uid       string          // authenticated user name
	Gids      map[string]bool // local groups known for that user
	SpeaksFor string          // user name as reported by the remote peer.
	Proto     map[string]bool // protocols spoken by the peer.
	Ok        bool            // auth was successful?
}

/*
	Per-user keys.
	See Info for a description.
*/
struct Key {
	Uid  string
	Gids []string
	Key  []byte
}

var (
	// Global certificates used by default for clients and servers.
	TLSclient, TLSserver   *tls.Config
	xTLSclient, xTLSserver *tls.Config

	// Paths to pem and key files used by servers.
	ServerPem, ServerKey string

	// Enable authentication. TLS can still be enabled with auth disabled.
	Enabled = true

	// Errors returned by authentication tools.
	ErrDisabled = errors.New("auth disabled")
	ErrTimedOut = errors.New("auth timed out")
	ErrFailed   = errors.New("auth failed")

	// Timeout placed on the authentication protocol.
	Tmout = 2 * time.Second

	// Enable debug diagnostics.
	Debug   bool
	dprintf = dbg.FlagPrintf(&Debug)

	chc  chan uint64
	keys []Key
	iv   []byte
)

/*
	Return true if the auth info indicates that the user is a member
	of the given group name or is the given group name.
	Everyone belongs to the empty ("") group.
	The nil auth info befongs to every group.
	ai.Uid "elf" belongs to every group.
*/
func (ai *Info) InGroup(name string) bool {
	return ai == nil || name == "" || ai.Uid == "elf" || name == ai.Uid || ai.Gids[name]
}

/*
	Used in tests to disable TLS.
	By convention, clive always enables TLS.
	When TLS is enabled all network connections are secured through TLS by default.
	Pipe and fifo connections are never encrypted, but they are also authenticated
	when authentication is enabled.
*/
func TLSenable(on bool) {
	if on {
		TLSclient, TLSserver = xTLSclient, xTLSserver
	} else {
		TLSclient = nil
		TLSserver = nil
	}
}

/*
	Build a TLS config for use with dialing functions provided by others.
*/
func TLScfg(pem, key string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(pem, key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}, nil
}

struct msg {
	enabled         bool
	user, speaksfor string
	proto           map[string]bool
	ch              []byte
}

func (m *msg) pack() []byte {
	var buf bytes.Buffer
	if m.enabled {
		buf.WriteString("auth")
	} else {
		buf.WriteString("noauth")
	}
	buf.WriteByte(0)
	buf.WriteString(m.user)
	buf.WriteByte(0)
	buf.WriteString(m.speaksfor)
	buf.WriteByte(0)
	buf.WriteByte(byte(len(m.proto)))
	for k := range m.proto {
		buf.WriteString(k)
		buf.WriteByte(0)
	}
	buf.Write(m.ch)
	return buf.Bytes()
}

func (m *msg) String() string {
	if m == nil {
		return "<nil auth msg>"
	}
	var buf bytes.Buffer
	if m.enabled {
		fmt.Fprintf(&buf, "auth")
	} else {
		fmt.Fprintf(&buf, "noauth")
	}
	fmt.Fprintf(&buf, " %s", m.user)
	for k := range m.proto {
		fmt.Fprintf(&buf, " %s", k)
	}
	return buf.String()
}

func unpackstr(buf []byte) (int, string, error) {
	for i := 0; i < len(buf); i++ {
		if buf[i] == 0 {
			return i + 1, string(buf[:i]), nil
		}
	}
	return 0, "", errors.New("auth: no string found")
}

func (m *msg) unpack(buf []byte) error {
	n, s, err := unpackstr(buf)
	if err != nil {
		return err
	}
	buf = buf[n:]
	m.enabled = s == "auth"
	n, m.user, err = unpackstr(buf)
	if err != nil {
		return err
	}
	buf = buf[n:]
	n, m.speaksfor, err = unpackstr(buf)
	if err != nil {
		return err
	}
	buf = buf[n:]
	if len(buf) < 1 {
		return errors.New("short auth msg")
	}
	np := int(buf[0])
	buf = buf[1:]
	m.proto = map[string]bool{}
	for i := 0; i < np; i++ {
		n, s, err = unpackstr(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
		m.proto[s] = true
	}
	m.ch = buf
	return nil
}

// Return the path to the directory where clive keys and certificates are kept.
func KeyDir() string {
	return path.Join(u.Home, ".ssh")
}

// Return the path to the file at dir where clive keys for the auth domain named are kept.
func KeyFile(dir, name string) string {
	if name == "" {
		name = "default"
	}
	return path.Join(dir, "clive."+name)
}

// Save the key for the given secret of the given user in the named auth domain
// at KeyFile(dir, name).
// The key is added if there is no such user in the auth domain or replaced
// if the user already exists.
func SaveKey(dir, name, user, secret string, groups ...string) error {
	if dir == "" {
		dir = KeyDir()
	}
	if name == "" {
		name = "default"
	}
	file := KeyFile(dir, name)
	data := []byte(secret)
	key := pbkdf2.Key(data, []byte("ltsa"), 1000, 32, sha1.New)

	old, _ := LoadKey(dir, name)
	new := []Key{}
	for _, o := range old {
		if o.Uid != user {
			new = append(new, o)
		}
	}
	new = append(new, Key{Uid: user, Gids: groups, Key: key})
	fd, err := os.Create(file)
	if err != nil {
		return err
	}
	for _, k := range new {
		if _, err := fmt.Fprintf(fd, "%s", k.Uid); err != nil {
			os.Remove(file)
			return err
		}
		for _, g := range k.Gids {
			if _, err := fmt.Fprintf(fd, " %s", g); err != nil {
				os.Remove(file)
				return err
			}
		}
		if _, err := fmt.Fprintf(fd, "\n%x\n", k.Key); err != nil {
			os.Remove(file)
			return err
		}
	}
	if err := fd.Close(); err != nil {
		os.Remove(file)
		return err
	}
	if err := os.Chmod(file, 0600); err != nil {
		os.Remove(file)
		return err
	}
	return nil
}

// Load the key for the named auth domain kept at dir. Return the user name for the key,
// the user key, and any error indication.
func LoadKey(dir, name string) (ks []Key, err error) {
	if dir == "" {
		dir = KeyDir()
	}
	if name == "" {
		name = "default"
	}
	file := path.Join(dir, "clive."+name)
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	scn := bufio.NewScanner(fd)
	for {
		if !scn.Scan() {
			if len(ks) == 0 {
				return nil, io.EOF
			}
			break
		}
		users := scn.Text()
		toks := strings.Fields(users)
		if len(toks) == 0 {
			return ks, io.EOF
		}
		user := toks[0]
		toks = toks[1:]
		if len(toks) == 0 {
			toks = append(toks, user)
		}
		if !scn.Scan() {
			return ks, io.EOF
		}
		key, err := hex.DecodeString(scn.Text())
		if err != nil {
			return ks, err
		}
		ks = append(ks, Key{Uid: user, Gids: toks, Key: key})
	}
	return ks, nil
}

/*
	Check out to see if resp is the expected response for the ch challenge on
	the named auth domain.
	Returns the user who authenticates and the status for authentication.
	Always returns true when Auth is not enabled.
*/
func ChallengeResponseOk(name, ch, resp string) (user string, ok bool) {
	usr := u.Uid
	if !Enabled {
		return usr, true
	}
	if keys == nil || iv == nil {
		return usr, false
	}
	usr = keys[0].Uid
	key := keys[0].Key
	if name != "" && name != "default" {
		var err error
		ks, err := LoadKey(KeyDir(), name)
		if err != nil {
			dbg.Warn("auth: loadkey %s: %s", name, err)
			return usr, false
		}
		usr, key = ks[0].Uid, ks[0].Key
	}
	chresp, ok := encrypt(key, iv, []byte(ch))
	if !ok || len(chresp) == 0 {
		return usr, false
	}
	return usr, fmt.Sprintf("%x", chresp) == resp
}

/*
	Run by a client to authenticate a connection to a server (as provided by clive/nchan).

	This performs symmetric challenge-response over a TLS secured connection
	(or a pipe or fifo). The shared key is kept at the KeyDir() directory in the
	file reported by KeyFile() for the named auth domain.

	By convention, the dialer takes the first key and user name kept in the key file
	and uses those. The callee waits until it has received a proposed user name
	to complete its part of the protocol, and selects the key for that user name
	as kept in the key file. The iscaller argument indicates if it's the dialer or not.

	If there's no key, or TLS is not configured for the network, or auth is not enabled, c is left
	undisturbed and an error is returned instead. The error is ErrDisabled when auth
	is disabled.

	Otherwise, c is closed unless it authenticates correctly and the auth info resulting
	from the protocol is returned.
*/
func AtClient(c ch.Conn, name string, proto ...string) (*Info, error) {
	return conn(c, true, name, true, proto...)
}

/*
	Like AtClient(), but with auth disabled for this client/server
*/
func NoneAtClient(c ch.Conn, name string, proto ...string) (*Info, error) {
	return conn(c, true, name, false, proto...)
}

/*
	Run by a server to authenticate a connection to a client (as provided by clive/nchan).
	Despite the names, the protocol is symmetric.
	See Caller for a description.
*/
func AtServer(c ch.Conn, name string, proto ...string) (*Info, error) {
	return conn(c, false, name, true, proto...)
}

/*
	Like AtServer(), but with auth disabled for this client/server
*/
func NoneAtServer(c ch.Conn, name string, proto ...string) (*Info, error) {
	return conn(c, false, name, false, proto...)
}

/*
	clive cr protocol:

	1.
	cli -> srv:	auth msg for uid with ch number
	srv -> cli: auth msg for uid with ch number (uid ignored here at client)

	2.
	cli checks srv auth, build common list of protocols and takes the challenge
	srv checks cli auth, build common list of protocols and scans for a user with
		the given name, then takes the challenge.

	if there's no auth in both sides, this is the end of the protocol

	if there's no auth in just one side, this is an error.

	3.
	cli responds for the ch sent from the server using its own uid
	srv responds for the ch sent from the client using the client's uid to encrypt

	4.
	cli checks the response, hangup or it's ok
	srv checks the response (using the client's uid), hangup or it's ok
*/
func conn(c ch.Conn, iscaller bool, name string, enabled bool, proto ...string) (*Info, error) {
	ch := make([]byte, 16)
	var k []byte
	user := u.Uid
	groups := []string{user}
	if keys != nil {
		user = keys[0].Uid
		k = keys[0].Key
	}
	enabled = enabled && Enabled
	if enabled {
		if TLSclient == nil || TLSserver == nil {
			return nil, errors.New("no tls")
		}
		if name != "" && name != "default" {
			var err error
			ks, err := LoadKey(KeyDir(), name)
			if err != nil {
				return nil, fmt.Errorf("no key: %s", err)
			}
			user, k, groups = ks[0].Uid, ks[0].Key, ks[0].Gids
		}
		if k == nil {
			return nil, errors.New("no key")
		}
		binary.LittleEndian.PutUint64(ch[0:], <-chc)
		binary.LittleEndian.PutUint64(ch[8:], <-chc)
	}

	// 1. send the auth msg and challenge
	m := &msg{enabled: enabled, user: user, speaksfor: u.Uid, ch: ch}
	m.proto = map[string]bool{}
	for _, s := range proto {
		m.proto[s] = true
	}
	dprintf("-> %s\n", m)
	tc := time.After(Tmout)
	select {
	case <-tc:
		close(c.Out, ErrTimedOut)
		close(c.In, ErrTimedOut)
		return nil, ErrTimedOut
	case c.Out <- m.pack():
		if cerror(c.Out) != nil {
			close(c.In, cerror(c.Out))
			return nil, cerror(c.Out)
		}
	}

	// 2. read the remote auth msg and challenge
	rm := &msg{}
	select {
	case <-tc:
		close(c.Out, ErrTimedOut)
		close(c.In, ErrTimedOut)
		return nil, ErrTimedOut
	case rdata := <-c.In:
		dat, ok := rdata.([]byte)
		var err error
		if !ok {
			err = errors.New("bad message")
		} else if err = rm.unpack(dat); err != nil {
			if cerror(c.In) != nil {
				err = fmt.Errorf("auth: %s", cerror(c.In))
			}
			err = fmt.Errorf("auth: %s", err)
			dprintf("<- %s\n", err)
			close(c.In, err)
			close(c.Out, err)
			return nil, err
		}
	}
	dprintf("<- %s\n", rm)

	info := &Info{
		Uid:       rm.user,
		SpeaksFor: rm.speaksfor,
		Proto:     rm.proto,
		Gids:      make(map[string]bool),
	}
	for k := range rm.proto {
		if !m.proto[k] {
			delete(rm.proto, k)
		}
	}
	if iscaller {
		info.Uid = user
	}

	switch {
	case len(rm.proto) == 0:
		err := errors.New("no shared protocol")
		close(c.In, err)
		close(c.Out, err)
		return info, err
	case !m.enabled && !rm.enabled:
		return info, nil
	case m.enabled != rm.enabled:
		close(c.In, ErrDisabled)
		close(c.Out, ErrDisabled)
		return info, ErrDisabled
	}

	// 3. respond (but server relies on the key for the user given by the caller).
	if !iscaller {
		k = nil
		groups = nil
		for _, key := range keys {
			if key.Uid == rm.user {
				k = key.Key
				user = key.Uid
				groups = key.Gids
			}
		}
		for _, g := range groups {
			info.Gids[g] = true
		}
		if k == nil {
			err := errors.New("wrong user/key")
			close(c.In, err)
			close(c.Out, err)
			return info, err
		}
	}
	resp, ok := encrypt(k, iv, rm.ch)
	if !ok {
		err := errors.New("encrypt failed")
		close(c.In, err)
		close(c.Out, err)
		return info, err
	}
	select {
	case <-tc:
		close(c.Out, ErrTimedOut)
		close(c.In, ErrTimedOut)
		return info, ErrTimedOut
	case c.Out <- resp:
		if cerror(c.Out) != nil {
			err := fmt.Errorf("auth: %s", cerror(c.Out))
			close(c.In, err)
			return info, err
		}
	}

	// 4. read the remote response
	var repl []byte
	select {
	case <-tc:
		close(c.Out, ErrTimedOut)
		close(c.In, ErrTimedOut)
		return info, ErrTimedOut
	case xrepl := <-c.In:
		repl, _ = xrepl.([]byte)
		if len(repl) == 0 {
			close(c.In, ErrFailed)
			close(c.Out, ErrFailed)
			return info, fmt.Errorf("%s: empty reply", ErrFailed)
		}
	}

	// check the response
	chresp, ok := encrypt(k, iv, m.ch[:])
	if !ok {
		err := errors.New("encrypt failed")
		close(c.In, err)
		close(c.Out, err)
		return info, err
	}

	if !bytes.Equal(chresp[:], repl[:]) {
		dbg.Warn("auth failed: %s (as %s)", info.SpeaksFor, info.Uid)
		close(c.In, ErrFailed)
		close(c.Out, ErrFailed)
		return info, fmt.Errorf("%s: bad reply", ErrFailed)
	}
	info.Ok = true
	return info, nil
}

// Pad applies the PKCS #7 padding scheme on the buffer.
func pad(in []byte) []byte {
	padding := 16 - (len(in) % 16)
	if padding == 0 {
		padding = 16
	}
	for i := 0; i < padding; i++ {
		in = append(in, byte(padding))
	}
	return in
}

// Unpad strips the PKCS #7 padding on a buffer. If the padding is
// invalid, nil is returned.
func unpad(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}

	padding := in[len(in)-1]
	if int(padding) > len(in) || padding > aes.BlockSize {
		return nil
	}

	for i := len(in) - 1; i > len(in)-int(padding); i-- {
		if in[i] != padding {
			return nil
		}
	}
	return in[:len(in)-int(padding)]
}

// Helper function.
// Decrypts the message and removes any padding.
func decrypt(k, in []byte) ([]byte, bool) {
	if len(in) == 0 || len(in)%aes.BlockSize != 0 {
		return nil, false
	}

	c, err := aes.NewCipher(k)
	if err != nil {
		return nil, false
	}

	cbc := cipher.NewCBCDecrypter(c, in[:aes.BlockSize])
	cbc.CryptBlocks(in[aes.BlockSize:], in[aes.BlockSize:])
	out := unpad(in[aes.BlockSize:])
	if out == nil {
		return nil, false
	}
	return out, true
}

func encrypt(k, iv, in []byte) ([]byte, bool) {
	in = pad(in)
	if iv == nil {
		return nil, false
	}

	c, err := aes.NewCipher(k)
	if err != nil {
		return nil, false
	}

	cbc := cipher.NewCBCEncrypter(c, iv)
	cbc.CryptBlocks(in, in)
	return in, true
}

func init() {
	dir := KeyDir()
	cli := path.Join(dir, "client")
	srv := path.Join(dir, "server")
	var err error
	TLSclient, err = TLScfg(cli+".pem", cli+".key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ds/auth: %s: %s\n", cli, err)
	} else {
		TLSclient.Rand = crand.Reader
	}
	ServerPem = srv + ".pem"
	ServerKey = srv + ".key"
	TLSserver, err = TLScfg(srv+".pem", srv+".key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ds/auth: %s: %s\n", srv, err)
	} else {
		TLSserver.Rand = crand.Reader
	}
	xTLSclient = TLSclient
	xTLSserver = TLSserver
	chc = make(chan uint64)
	go func() {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			chc <- uint64(r.Int63())
		}
	}()

	keys, err = LoadKey(dir, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "net/auth: loadkey: %s\n", err)
		return
	}
	iv, err = hex.DecodeString("12131415161718191a1b1c1d1e1f1011")
	if err != nil {
		panic(err)
	}
}

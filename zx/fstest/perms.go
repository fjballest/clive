package fstest

import (
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"fmt"
	"strings"
)

func authfs(t Fataler, xfs zx.Tree) zx.RWTree {
	d, err := zx.Stat(xfs, "/d")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	printf("d is: %s\n", d)
	uid := d["Uid"]
	if uid == "" {
		t.Fatalf("no uid")
	}
	gid := d["Gid"]
	if uid == "" {
		t.Fatalf("no gid")
	}
	wuid := d["Wuid"]
	if wuid == "" {
		t.Fatalf("no wuid")
	}
	ai := &auth.Info{
		Uid: uid, SpeaksFor: uid, Ok: true,
		Gids: map[string]bool{"gid1": true, "gid2": true},
	}
	if gid != uid {
		ai.Gids[gid] = true
	}
	return authfor(t, xfs, ai)
}

func authfor(t Fataler, xfs zx.Tree, ai *auth.Info) zx.RWTree {
	afs, ok := xfs.(zx.AuthTree)
	if !ok {
		t.Fatalf("tree is not an auth tree")
	}
	aifs, err := afs.AuthFor(ai)
	if err != nil {
		t.Fatalf("auth: %s", err)
	}
	fs, ok := aifs.(zx.RWTree)
	if !ok {
		t.Fatalf("auth'ed fs is not a rw tree")
	}
	return fs
}

func stat(t Fataler, fs zx.Tree, p, res string) bool {
	d, err := zx.Stat(fs, p)
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	s := fmt.Sprintf("%s %s %s %s", d["mode"], d["Uid"], d["Gid"], d["Wuid"])
	us := strings.Replace(s, dbg.Usr, "nemo", -1)
	printf("%s: %s\n", p, us)

	// At least lfs does not update Wuid for directories, but that's ok
	if d["type"] == "d" && res != "" {
		toks := strings.Fields(res)
		res = strings.Join(toks[:len(toks)-1], " ")
		s = fmt.Sprintf("%s %s %s", d["mode"], d["Uid"], d["Gid"])
		us = strings.Replace(s, dbg.Usr, "nemo", -1)
	}
	if res != "" && us != res {
		t.Logf("wrong stat for %s <%s>", p, us)
		return false
	}
	return true
}

func nostat(t Fataler, fs zx.Tree, p string) bool {
	_, err := zx.Stat(fs, p)
	if err == nil {
		t.Logf("could stat %s", p)
		return false
	}
	return true
}

type usr struct {
	who string
	ai  *auth.Info
}

// see if fn has errors (permissions) or not for the given path and 1,2,4 perm. bit.
// returns xxxxxxxxxxxx where x is y or n
// for ugo and masks 777 770 700 000
// Expected result is always yyyyynynnnnn
func ugo(t Fataler, fs zx.RWTree, p string, bit uint, fn func(zx.RWTree) error) string {
	err := <-fs.Wstat(p, zx.Dir{"mode": "0777"})
	if err != nil {
		t.Fatalf("wstat: %s", err)
	}
	defer func() {
		// so we can later remove it
		<-fs.Wstat(p, zx.Dir{"mode": "0755"})
	}()
	// /chkd is nemo gid1
	usrs := []usr{
		usr{who: "uid", ai: &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true,
			Gids: map[string]bool{}}},
		usr{who: "gid", ai: &auth.Info{Uid: "gid1", SpeaksFor: dbg.Usr, Ok: true,
			Gids: map[string]bool{}}},
		usr{who: "oid", ai: &auth.Info{Uid: "other", SpeaksFor: dbg.Usr, Ok: true,
			Gids: map[string]bool{}}},
	}
	shift := uint(0)
	m := uint(0777)
	out := ""
	for i := 0; i < 4; i++ {
		printf("mode %o\n", m)
		err := <-fs.Wstat(p, zx.Dir{"mode": fmt.Sprintf("0%o", m)})
		if err != nil {
			t.Fatalf("wstat: %s", err)
		}
		for _, u := range usrs {
			if err := fn(authfor(t, fs, u.ai)); err != nil {
				out += "n"
				printf("%s can't for %o\n", u.who, m)
			} else {
				out += "y"
				printf("%s can for %o\n", u.who, m)
			}
		}
		m &^= (bit << shift)
		shift += 3
	}
	return out
}

// Check that perms are actually checked in old files and dirs
func RWXPerms(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := authfs(t, fss[0].(zx.RWTree))

	if <-fs.Mkdir("/chkd", zx.Dir{"mode": "0777", "Gid": "gid1"}) != nil {
		t.Fatalf("couldn't mkdir /chkd gid")
	}
	if <-fs.Mkdir("/chkd/o", zx.Dir{"mode": "0777", "Gid": "gid1"}) != nil {
		t.Fatalf("couldn't mkdir /chkd gid")
	}

	// Check out dirs

	rfn := func(fs zx.RWTree) error {
		_, err := zx.GetDir(fs, "/chkd")
		return err
	}
	out := ugo(t, fs, "/chkd", 4, rfn)
	printf("dir rd ugo: %s\n", out)
	if out != "yyyyynynnnnn" {
		t.Fatalf("dir read perms are %s", out)
	}
	wfn := func(fs zx.RWTree) error {
		err := <-fs.Mkdir("/chkd/x", zx.Dir{"mode": "0777"})
		if err != nil {
			return err
		}
		return <-fs.Remove("/chkd/x")
	}
	out = ugo(t, fs, "/chkd", 2, wfn)
	printf("dir wr ugo: %s\n", out)
	if out != "yyyyynynnnnn" {
		t.Fatalf("dir write perms are %s", out)
	}

	xfn := func(fs zx.RWTree) error {
		_, err := zx.GetDir(fs, "/chkd/o")
		return err
	}
	out = ugo(t, fs, "/chkd", 1, xfn)
	printf("dir ex ugo: %s\n", out)
	if out != "yyyyynynnnnn" {
		t.Fatalf("dir exec perms are %s", out)
	}

	// Check out files
	if err := zx.PutAll(fs, "/chkf", zx.Dir{"mode": "0777"}, []byte("hi")); err != nil {
		t.Fatalf("put: %s", err)
	}
	if err := <-fs.Wstat("/chkf", zx.Dir{"Gid": "gid1"}); err != nil {
		t.Fatalf("wstat: %s", err)
	}
	rfn = func(fs zx.RWTree) error {
		_, err := zx.GetAll(fs, "/chkf")
		return err
	}
	out = ugo(t, fs, "/chkf", 4, rfn)
	printf("file rd ugo: %s\n", out)
	if out != "yyyyynynnnnn" {
		t.Fatalf("file read perms are %s", out)
	}

	wfn = func(fs zx.RWTree) error {
		return zx.PutAll(fs, "/chkf", nil, []byte("there"))
	}
	out = ugo(t, fs, "/chkf", 2, wfn)
	printf("file wr ugo: %s\n", out)
	if out != "yyyyynynnnnn" {
		t.Fatalf("file write perms are %s", out)
	}
	// We can't check out exec perms, because the underlying OS has
	// its own idea of who can exec the file.

	// see who can change modes.
	m := 0
	wfn = func(fs zx.RWTree) error {
		m++
		return <-fs.Wstat("/chkf", zx.Dir{"mode": fmt.Sprintf("0%o", m)})
	}
	out = ugo(t, fs, "/chkf", 2, wfn)
	printf("file wstat ugo: %s\n", out)
	if out != "ynnynnynnynn" {
		t.Fatalf("file wstat perms are %s", out)
	}

}

// Check perms, uids, and gids for new files and dirs
// including inheriting bits and uids and that we can't change uids that we can't change.
func NewPerms(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := authfs(t, fss[0].(zx.RWTree))

	// This is a contorted dance to check that we can/can't change uids/gids/wuids
	// depending on the original uids and permissions for non-existing files,
	// existing files, and dirs.
	// It checks also that gids and perms are inherited from the containing dir
	// as they should

	if err := <-fs.Wstat("/d", zx.Dir{"Gid": "katsumoto"}); err == nil {
		t.Fatalf("could wstat gid")
	}
	if err := <-fs.Wstat("/d", zx.Dir{"Gid": "gid1"}); err != nil {
		t.Fatalf("wstat: %s", err)
	}
	if err := <-fs.Wstat("/d", zx.Dir{"Uid": "katsumoto"}); err == nil {
		t.Fatalf("could wstat uid")
	}
	if err := <-fs.Wstat("/d", zx.Dir{"Wuid": "katsumoto"}); err == nil {
		t.Fatalf("could wstat wuid")
	}
	if !stat(t, fs, "/d", "0755 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if err := <-fs.Wstat("/d", zx.Dir{"Uid": "gid2"}); err != nil {
		t.Fatalf("wstat: %s", err)
	}
	if !stat(t, fs, "/d", "0755 gid2 gid1 nemo") {
		t.Fatalf("stat")
	}
	if err := <-fs.Wstat("/d", zx.Dir{"Uid": dbg.Usr}); err != nil {
		t.Fatalf("couldn't wstat uid")
	}
	if err := <-fs.Wstat("/d", zx.Dir{"mode": "0750"}); err != nil {
		t.Fatalf("couldn't wstat mode")
	}
	if !stat(t, fs, "/d", "0750 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	dat := []byte("hi")
	if zx.PutAll(fs, "/d/newauth", zx.Dir{"mode": "0604", "Uid": "katsumoto"}, dat) == nil {
		t.Fatalf("could put uid on create")
	}
	if !nostat(t, fs, "/d/newauth") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth2", zx.Dir{"mode": "0604", "Gid": "katsumoto"}, dat) == nil {
		t.Fatalf("could put gid on create")
	}
	if !nostat(t, fs, "/d/newauth2") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth3", zx.Dir{"mode": "0604", "Wuid": "katsumoto"}, dat) != nil {
		t.Fatalf("put wuid not ignored on create")
	}
	if !stat(t, fs, "/d/newauth3", "0640 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth3", zx.Dir{"mode": "0604", "Uid": "katsumoto"}, dat) == nil {
		t.Fatalf("could put uid on existing")
	}
	if !stat(t, fs, "/d/newauth3", "0640 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth3", zx.Dir{"mode": "0604", "Gid": "katsumoto"}, dat) == nil {
		t.Fatalf("could put gid on existing")
	}
	if !stat(t, fs, "/d/newauth3", "0640 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth3", zx.Dir{"mode": "0604", "Wuid": "katsumoto"}, dat) != nil {
		t.Fatalf("put wuid not ignored on existing")
	}
	if !stat(t, fs, "/d/newauth3", "0640 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth4", zx.Dir{"mode": "0604", "Uid": "gid1"}, dat) != nil {
		t.Fatalf("couldn't put uid for gid on create")
	}
	if !stat(t, fs, "/d/newauth4", "0640 gid1 gid1 nemo") {
		t.Fatalf("stat")
	}

	if zx.PutAll(fs, "/d/newauth3", zx.Dir{"mode": "0604", "Uid": "gid1", "Gid": "gid2"}, dat) != nil {
		t.Fatalf("couldn't put uid/gid on existing")
	}
	if !stat(t, fs, "/d/newauth3", "0640 gid1 gid2 nemo") {
		t.Fatalf("stat")
	}

	if <-fs.Mkdir("/d/newd", zx.Dir{"mode": "0755", "Uid": "katsumoto"}) == nil {
		t.Fatalf("could mkdir uid")
	}
	if !nostat(t, fs, "/d/newd") {
		t.Fatalf("stat")
	}

	if <-fs.Mkdir("/d/newd", zx.Dir{"mode": "0705", "Uid": "gid2"}) != nil {
		t.Fatalf("couldn't mkdir uid")
	}
	if !stat(t, fs, "/d/newd", "0750 gid2 gid1 nemo") {
		t.Fatalf("stat")
	}

	if <-fs.Mkdir("/d/newd2", zx.Dir{"mode": "0705", "Gid": "katsumoto"}) == nil {
		t.Fatalf("could mkdir gid")
	}
	if !nostat(t, fs, "/d/newd2") {
		t.Fatalf("stat")
	}

	if <-fs.Mkdir("/d/newd2", zx.Dir{"mode": "0705", "Gid": "gid2"}) != nil {
		t.Fatalf("couldn't mkdir gid")
	}
	if !stat(t, fs, "/d/newd2", "0750 nemo gid2 nemo") {
		t.Fatalf("stat")
	}

	if <-fs.Mkdir("/d/newd3", zx.Dir{"mode": "0705", "Wuid": "katsumoto"}) != nil {
		t.Fatalf("mkdir wuid not ignored")
	}
	if !stat(t, fs, "/d/newd3", "0750 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

	if <-fs.Wstat("/d/newd3", zx.Dir{"mode": "0705"}) != nil {
		t.Fatalf("wstat 755")
	}
	if !stat(t, fs, "/d/newd3", "0705 nemo gid1 nemo") {
		t.Fatalf("stat")
	}

}

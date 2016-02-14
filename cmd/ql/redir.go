package main

import (
	"clive/ch"
	"clive/cmd"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"os"
	fpath "path"
	"strings"
)

func fields(s, sep string) []string {
	toks := strings.Split(s, sep)
	for i := 0; i < len(toks); i++ {
		toks[i] = strings.TrimSpace(toks[i])
	}
	return toks
}

// Args[0] is "<", ">", ">>"
// tag is Args[1] and can what's within [] in:
// >[out,err]
// <[in]
// |[in:out,err;in2:out2]
func (nd *Nd) addRedirTo(set []*Redir) []*Redir {
	nd.chk(Nredir)
	if len(nd.Args) != 2 {
		panic("addRedirTo: bad redir Args")
	}
	if len(nd.Child) > 1 {
		panic("addRedirTo: bad redir children")
	}
	what, tag := nd.Args[0], nd.Args[1]
	bad := ":;,$"
	switch what {
	case ">:":
		bad = ";,$"
	case ">", ">>":
		bad = ":;$"
	case "<|", ">|":
		bad = "$"
	}
	if strings.ContainsAny(tag, bad) {
		yylex.Errs("bad %s redirection syntax '%s'", what, tag)
		panic(parseErr)
	}
	if what == ">:" {
		flds := fields(tag, ":")
		if len(flds) == 1 {
			flds = append(flds, "out")
			nd.Args[0] += ":out"
		}
		if len(flds) != 2 {
			yylex.Errs("bad > redirection syntax '%s'", tag)
			panic(parseErr)
		}
		set = append(set, &Redir{name: strings.Join(flds, ":")})
		return set
	}
	for _, r := range fields(tag, ",") {
		for _, rd := range set {
			if rd.name == r {
				yylex.Errs("double redirection for '%s' in '%s' ", r, tag)
				panic(parseErr)
			}
		}
		set = append(set, &Redir{name: r, nd: nd})
	}
	return set
}

// Called to add the redirs implied by a pipe
func (nd *Nd) addPipeRedirs(stdin bool) {
	nd.chk(Npipe)
	nc := len(nd.Child)
	if nc == 0 {
		panic("addPipeRedirs: no command 0\n")
	}
	if len(nd.Args) != nc {
		panic("addPipeRedirs: bad pipe Args")
	}
	c0 := nd.Child[0]
	if stdin {
		r := newRedir("<", "in", newNd(Nname, "<"))
		c0.Redirs = r.addRedirTo(c0.Redirs)
	}
	if nc == 1 {
		// single command, not really a pipe
		return
	}
	tags := nd.Args[1:] // 1st arg is the bg tag
	for i, tag := range tags {
		tags := fields(tag, ";")
		for _, tag := range tags {
			rdrs := fields(tag, ":")
			if len(rdrs) == 1 {
				rdrs = append([]string{"in"}, rdrs[0])
			}
			if len(rdrs)%2 != 0 {
				yylex.Errs("syntax error in redir '%s'", tag)
				panic(parseErr)
			}
			for n := 0; n < len(rdrs); n += 2 {
				name := newNd(Nname, fmt.Sprintf("|%d", i))
				r := newRedir("<|", rdrs[n], name)
				nd.Child[i+1].Redirs = r.addRedirTo(nd.Child[i+1].Redirs)
				r = newRedir(">|", rdrs[n+1], name)
				nd.Child[i].Redirs = r.addRedirTo(nd.Child[i].Redirs)
			}
		}
	}
}

// what is "<", ">", ">>"
// tag can be "" or "in", "out", "in,out,foo,..."
// nd is the target of the redir
// name can be nil for >, in which case it's a dup.
func newRedir(what, tag string, name *Nd) *Nd {
	tag = strings.TrimSpace(tag)
	if what == ">" && name == nil {
		what = ">:"
	}
	if what != ">:" && name == nil {
		yylex.Errs("missing name for '%s' ", what)
		panic(parseErr)
	}
	if tag == "" {
		if what == "<" || what == "<|" {
			tag = "in"
		} else {
			tag = "out"
		}
	}
	nd := newNd(Nredir, what, tag)
	if name != nil {
		nd.Add(name)
	}
	return nd
}

// Used by redirs
// We rely zx through a pipe, so unix commands could be happy.
// Only []byte messages are relayed.
func inFrom(path string) (*os.File, error) {
	name, pred := cmd.CleanName(path)
	if pred == "0" {
		if _, err := cmd.Stat(name); err != nil {
			return nil, err
		}
		rd, wr, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		rc := cmd.Get(path, 0, -1)
		go func() {
			defer wr.Close()
			for m := range rc {
				if _, err := ch.WriteMsg(wr, 1, m); err != nil {
					close(rc, err)
					return
				}
			}
			if err := cerror(rc); err != nil {
				cmd.Warn("<: %s: %s", path, err)
			}
		}()
		return rd, nil
	}
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer wr.Close()
		rc := cmd.Files(path)
		_, _, err := ch.WriteMsgs(wr, 1, rc)
		if err != nil {
			close(rc, err)
		}
	}()
	return rd, nil
}

// The returned chan is used by the command environment to wait for the
// writes to complete, because this is a zx stream now.
func outTo(path string, app bool) (*os.File, chan bool, error) {
	name, pred := cmd.CleanName(path)
	if pred != "0" {
		return nil, nil, errors.New("can't use predicates for > redir")
	}
	ppath := fpath.Dir(name)
	d, err := cmd.Stat(ppath)
	if err != nil {
		return nil, nil, err
	}
	if d["type"] != "d" {
		return nil, nil, fmt.Errorf("%s: %s", path, zx.ErrNotDir)
	}
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	dc := make(chan bool)
	go func() {
		defer func() {
			rd.Close()
			close(dc)
		}()
		buf := make([]byte, ch.MsgSz)
		dc := make(chan []byte)
		var d zx.Dir
		var off int64
		if app {
			off = -1
		} else {
			d = zx.Dir{"type": "-"}
		}
		rc := cmd.Put(path, d, off, dc)
		for {
			n, rerr := rd.Read(buf[0:])
			if rerr != nil {
				if rerr != io.EOF && err == nil {
					err = rerr
				}
				break
			}
			if err != nil {
				continue
			}
			m := make([]byte, n)
			copy(m, buf[:n])
			if ok := dc <- m; !ok {
				err = cerror(dc)
				cmd.Warn(">: write: %s", err)
			}
		}
		close(dc)
		<-rc
		if err := cerror(rc); err != nil {
			cmd.Warn(">: write: %s", cerror(dc))
		}
	}()
	return wr, dc, nil
}

// Called for each pipe child to apply its redirs, including those for the pipeline
func (c *Nd) applyRedirs(x, cx *xEnv, pipes map[string]pFd) ([]io.Closer, error) {
	var pcloses []io.Closer
	for _, rd := range c.Redirs {
		r := rd.nd
		if r == nil { // dup
			flds := fields(rd.name, ":")
			nfd, ofd := flds[0], flds[1]
			xfd := cx.fds[ofd]
			if xfd != nil {
				xfd.ref++
			}
			if fd, ok := cx.fds[nfd]; ok {
				fd.Close()
			}
			cx.fds[nfd] = xfd
			continue
		}
		paths, err := r.Child[0].expand1(x)
		if err != nil {
			cmd.Warn("expand: %s", err)
			return pcloses, err
		}
		path := paths[0]
		kind, tag := r.Args[0], r.Args[1]
		var osfd *os.File
		var dc chan bool
		cnames := fields(tag, ",")
		switch kind {
		case "<":
			osfd, err = inFrom(path)
			if err != nil {
				cmd.Warn("redir: %s", err)
				return pcloses, err
			}
			pcloses = append(pcloses, osfd)
		case ">":
			osfd, dc, err = outTo(path, false)
			// osfd, err = os.Create(path)
			if err != nil {
				cmd.Warn("redir: %s", err)
				return pcloses, err
			}
			pcloses = append(pcloses, osfd)
			cx.waits = append(cx.waits, dc)
		case ">>":
			osfd, dc, err = outTo(path, true)
			// osfd, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				cmd.Warn("redir: %s", err)
				return pcloses, err
			}
			pcloses = append(pcloses, osfd)
			cx.waits = append(cx.waits, dc)
		case "<|", ">|":
			p, ok := pipes[path]
			if !ok {
				p.r, p.w, err = os.Pipe()
				if err != nil {
					cmd.Warn("pipe: %s", err)
					return pcloses, err
				}
				pcloses = append(pcloses, p.r, p.w)
				pipes[path] = p
			}
			if kind[0] == '>' {
				osfd = p.w
			} else {
				osfd = p.r
			}
		default:
			panic("bad kind")
		}
		isin := kind[0] == '<'
		xfd := &xFd{fd: osfd, path: path, ref: 0, isIn: isin}
		for _, cname := range cnames {
			xfd.ref++
			if fd, ok := cx.fds[cname]; ok {
				fd.Close()
			}
			cx.fds[cname] = xfd
		}
	}
	return pcloses, nil
}

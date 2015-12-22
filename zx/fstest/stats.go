package fstest

import (
	"clive/zx"
	"strings"
)

var (
	stats = []string {
`name:"/" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/" addr:"lfs!local!/tmp/zx_test"`,
`name:"a" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/a" addr:"lfs!local!/tmp/zx_test/a"`,
`name:"b" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/a/b" addr:"lfs!local!/tmp/zx_test/a/b"`,
`name:"c" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/a/b/c" addr:"lfs!local!/tmp/zx_test/a/b/c"`,
`name:"d" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/d" addr:"lfs!local!/tmp/zx_test/d"`,
`name:"e" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/e" addr:"lfs!local!/tmp/zx_test/e"`,
`name:"f" type:"d" mode:"0755" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/e/f" addr:"lfs!local!/tmp/zx_test/e/f"`,
`name:"1" type:"-" mode:"0644" size:"0" uid:"elf" gid:"root" wuid:"elf" path:"/1" addr:"lfs!local!/tmp/zx_test/1"`,
`name:"a1" type:"-" mode:"0644" size:"10154" uid:"elf" gid:"root" wuid:"elf" path:"/a/a1" addr:"lfs!local!/tmp/zx_test/a/a1"`,
`name:"a2" type:"-" mode:"0644" size:"21418" uid:"elf" gid:"root" wuid:"elf" path:"/a/a2" addr:"lfs!local!/tmp/zx_test/a/a2"`,
`name:"c3" type:"-" mode:"0644" size:"44970" uid:"elf" gid:"root" wuid:"elf" path:"/a/b/c/c3" addr:"lfs!local!/tmp/zx_test/a/b/c/c3"`,
`name:"2" type:"-" mode:"0644" size:"31658" uid:"elf" gid:"root" wuid:"elf" path:"/2" addr:"lfs!local!/tmp/zx_test/2"`,
}

)

func Stats(t Fataler, fs zx.Fs) {
	ds := []zx.Dir{}
	for _, p := range AllFiles {
		dc := fs.Stat(p)
		d := <-dc
		if err := cerror(dc); err != nil {
			t.Fatalf("stat %s: %s", p, err)
		}
		ds = append(ds, d)
	}

	for _, d := range ds {
		Printf("%s\n", d.Fmt())
	}
	Printf("\n")
	for _, d := range ds {
		Printf("%s\n", d.LongFmt())
	}
	Printf("\n")
	for i, d := range ds {
		s := d.TestFmt()
		Printf("`%s`,\n", s)
		if !strings.HasPrefix(stats[i], s) {
			t.Fatalf("bad stat")
		}
	}
	Printf("\n")

	for _, p := range NotThere {
		dc := fs.Stat(p)
		d := <-dc
		if d != nil {
			t.Fatalf("%s is there", p)
		}
		if !zx.IsNotExist(cerror(dc)) {
			Printf("bad err %v\n", cerror(dc))
		}
	}

	for i, p := range BadPaths {
		d, err := zx.Stat(fs, p)
		if d != nil {
			Printf("%s\n", d.TestFmt())
		} else {
			Printf("err %s\n", err)
		}
		if i == 0 && d["path"] != "/" {
			t.Fatalf("bad bad path")
		}
		if i > 0 && d != nil {
			t.Fatalf("bad bad path")
		}
	}
}

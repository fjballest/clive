#!/bin/sh

#http://www.grsites.com/archive/sounds/category/21/?offset=3
#http://soundbible.com/free-sound-effects-8.html
#http://www.pdsounds.org/library_abc/B/Beeping

hget http://static1.grsites.com/archive/sounds/scifi/scifi019.mp3 > woosh.mp3
hget	http://www.pdsounds.org/audio/download/209/bip.mp3 > beep.mp3
hget http://static1.grsites.com/archive/sounds/farm/farm009.mp3 > sheep.mp3
hget http://static1.grsites.com/archive/sounds/scifi/scifi048.mp3 > phaser.mp3
hget http://static1.grsites.com/archive/sounds/aircraft/aircraft008.mp3 > rocket.mp3
hget 'http://soundbible.com/grab.php?id=2044&type=mp3' > tick.mp3


hget http://jetcityorange.com/musical-notes/C4-261.63.mp3> cnote.mp3
hget http://jetcityorange.com/musical-notes/Csharp4-277.18.mp3> csharpnote.mp3
hget http://jetcityorange.com/musical-notes/D4-293.66.mp3> dnote.mp3
hget http://jetcityorange.com/musical-notes/Dsharp4-311.13.mp3> dsharpnote.mp3
hget http://jetcityorange.com/musical-notes/E4-329.63.mp3> enote.mp3

hget http://jetcityorange.com/musical-notes/F4-349.23.mp3 > fnote.mp3
hget http://jetcityorange.com/musical-notes/Fsharp4-369.99.mp3 > fsharpnote.mp3
hget http://jetcityorange.com/musical-notes/G4-392.0.mp3 > gnote.mp3
hget http://jetcityorange.com/musical-notes/Gsharp4-415.3.mp3 >gsharpnote.mp3
hget http://jetcityorange.com/musical-notes/A4-440.0.mp3  > anote.mp3
hget http://jetcityorange.com/musical-notes/Asharp4-466.16.mp3 > asharpnote.mp3
hget http://jetcityorange.com/musical-notes/B4-493.88.mp3 > bnote.mp3

# hget http://soundbible.com/grab.php?id=1847&type=mp3 >/tmp/bomb.mp3 EDITED
hget 'http://soundbible.com/grab.php?id=1830&type=mp3' > fail.mp3
hget 'http://www.pslib.cz/ondrej.malik/L4_video/NNOChallenge/zvuky/TaDa.mp3' > tada.mp3

convertogo(){
	f=$1
	hdr="package gx; var $f = []byte {"
	{
	echo $hdr;
	cat $f.mp3 |xd -bx|9 sed 's/^.......( +)//g' |awk '/./' |9 sed 's/( |$)/, /g'| 9 sed 1247q | 9 sed 's/([0-9a-f][0-9a-f])/0x\1/g';
	echo '}';
	} > "../$f.go"
}

convertogo "woosh"
convertogo "beep"
convertogo "sheep"
convertogo "phaser"
convertogo "rocket"
convertogo anote
convertogo asharpnote
convertogo bnote
convertogo cnote
convertogo csharpnote
convertogo dnote
convertogo dsharpnote
convertogo enote
convertogo fnote
convertogo fsharpnote
convertogo gnote
convertogo gsharpnote
convertogo bomb
convertogo fail
convertogo tada


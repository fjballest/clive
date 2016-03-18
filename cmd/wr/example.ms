.\" grap example.ms | pic  | tbl | eqn | groff -Tutf8 -ms -m pspic  |pstopdf -i -o  example.pdf

.TL
title perhaps continued here with \f(CWverb \e\fP text
.AU
author info
.br
more author info with \f(CWtt text\fP

.AB
A section perhaps with the abstract if it's titled "Abstract". Tildes:
Axéáñññ

.AE
.NH
A sect
.LP
.NH 2
A subsect
.LP
Use 1, or any keys, to refer to the section with the title containing
all the words.

.NH 3
A sub-sub sect
.LP
\&.abc escaped

\&'abc escaped _

"cd escaped"

%tex comment scaped

some text making one paragraph
.br
with a forced line break

\f(CWverb\fP at the start of a par.

another paragraph

.P
.RS
To relatively indent a paragraph, use the tab.

.RE
This executes a command and takes its output as verbatim text:
.P
.RS
.DS
.CW
.ps -2
Thu Mar 17 15:51:11 CET 2016
Thu Mar 17 15:51:11 CET 2016
.ps +2
.R
.DE

.RE
This is the same but uses rc as a shell
.P
.RS
.DS
.CW
.ps -2
/Users/nemo/bin /usr/local/go/bin /usr/local/bin /usr/bin /bin /usr/sbin /sbin /plan9/bin /zx/usr/nemo/bin/rc /zx/usr/nemo/bin/darwin_amd64 /zx/bin/rc /zx/bin/darwin_amd64 .
.ps +2
.R
.DE

.RE
For rc and ql as shells we can just say
.P
.RS
.DS
.CW
.ps -2
ql
.ps +2
.R
.DE

.RE
yet another paragraph
.P
.RS
with closely indented par
.RE
within other text

this paragraph includes a list of items
.P
.RS
.IP \(bu
item 1 starts here and continues with more text

another paragrah within the same item, starting a new itemize
.P
.RS
.IP \(bu
2nd level 1st item
.IP \(bu
2nd level 2nd item
.RE
.IP \(bu
item 2 text continued here
.IP \(bu
item 3 text using multiple lines

.RE
after this paragraph comes some indented text with a list
.br
of numbered items and another list of items.

.P
.RS
some indented text
.RE
.P
.RS
.IP 1.
first numbered item
.IP 2.
another
.IP 3.
yet another
.RE
.P
.RS
.IP \(bu
first item
.IP \(bu
last item

.RE
A description list is a list of items where each item ends within the
first line and is followed by indented paragraphs describing the item.
It is ok if the entire description name (item) is marked with a font
indicator, but items should be simple names and cannot have font
switches or other markups in the middle.
.P
.RS
\(bu \fB\fIfirst description\fP\fP
.RS
for this and that

with a second paragraph.

.RE
\(bu \fB\f(CW/a/b/c\fP\fP
.RS
for this other thing
.RE
\(bu \fB-f\fP
.RS
for another thing

.RE
.RE
this text has some \fBtext set in bold face\fP and some in
\fIitalic\fP, and some in \f(CWteletype font\fP.

Within teletype font \f(CWno character is special, including _ and
*\fP. This is good for things like \f(CW/a/b/c\fP.

You can repeat the * to scape it, or _ or |.

you can use
.ps +2
(or + any other number) to increase to a bigger font size and
.ps -2
to return to the normal size, and you can use
.I
to italize a set of lines
.R
or start an
.P
.RS
indented paragraph and then
.B
make a set of lines bold
.R
.RE
or
.CW
make a set of lines fixed font
.R

this paragraph has
.P
.RS
.DS
.CW
.ps -2
\&.text starting with dot
some indented verbatim text, including ['s and ]'s and \en
nb = 0
[
    pf = pf.Close()
]
wrelems(out, e.Child, lvl+1)
.ps +2
.R
.DE
.RE
but the verbatim text could be unindented as well.

this other paragraph has a cite [for this] and a url for the lsub page
[http://lsub.org] within the rest of the text you could also place a
link to [http://google.com]. Imprecise citations like [1,2,2] can also
be made. Cites like intro(1) refer to the manual.

.P
.RS
.KF
.PSPIC logols.eps
.RS
\fBFigure 1:\fP \fICaption goes here and perhaps continues here.
\fP
.RE
.KE

.RE
or write pic directly in place

.KF
.PS
circle "hola"
arrow
circle "caracola"
.PE
.RS
\fBFigure 2:\fP \fIthe caption starts without indentation and may
span several lines. \fP
.RE
.KE

Fig is number 2.

Use 1 for tables, 1 for eqns, and key for listings. Do not use these
references at the start of text in a line or they may be understood as
an actual table, etc.

for tables and equations we can use
.KF
.TS
center allbox;
cB cB cB
lB c r.
	col2	col3
row1	11	12
row2	21	22
.TE
.RS
\fBTable 1:\fP \fItables may have captions. The first line of items
always describes column formats, the second line always describes
columns, and the first column always describes rows. No other table
formats are supported. \fP
.RE
.KE

.KF
.EQ
a sub 3 sup 5

.EN
.RS
\fBEqn. 1:\fP \fIeqns may have captions \fP
.RE
.KE

.KF
.DS
.CW
.ps -2
{
    some prog or code
    taken verbatim to be printed
}
.ps +2
.R
.DE
.RS
\fBListing 1:\fP \fIit may have caption, the word after [code is
used as a tag in the listing. the default is program. but all code
listings share the same code counter despite the tag used. You can use
\f(CWmarks\fP and [cites] here. \fP
.RE
.KE

.KF
.G1
draw solid
1896	54.2
1900	49.4
1904	49.2
1908	50.0
1912	48.2
.G2
.RS
\fBFigure 3:\fP \fIA plot of data using grap for pic \fP
.RE
.KE

and so on...

.NH
See also
.LP
.P
.RS
.IP \(bu
intro(1) for so and so.
.RE
.SH
References
.LP
.SM
1. The Organization of Networks in Plan 9. Dave Presotto,Phil
Winterbottom. Plan 9 User's Manual. Vol. 2.
.br
2. Plan B User's Manual. Third edition. Laboratorio de Systemas, URJC.
GSYC-TR-2005-04. Also at http://planb.lsub.org/sys/man. 2005.
.br
.NS

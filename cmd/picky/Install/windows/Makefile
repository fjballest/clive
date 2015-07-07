# Use one of:
#	cygwin, windows


#NSIS=$(shell cygpath -p "${PROGRAMFILES}\NSIS\makensis.exe")
NSIS=makensis

all:	picky.res picky.exe

install: all

picky.ico:
	cp ../icons/picky.ico .

picky.res: picky.rc picky.ico
	echo in windows only
	echo windres picky.rc -O coff -o picky.res
	echo cp picky.res ../icons

picky.exe: picky.res
	cp ../icons/picky.ico .
	cp $$GOBIN/"$$sysos"_"$$mach"/pam.exe pam.exe
	cp $$GOBIN/"$$sysos"_"$$mach"/pick.exe pick.exe
	"$(NSIS)" picky.nsi	
	mv picky.exe ../bin/picky"$$mach".exe

clean:
	rm -rf picky.res picky.exe pam.exe pick.exe  ./*~ picky.ico

.PHONY: all clean install

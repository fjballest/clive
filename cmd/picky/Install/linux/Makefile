NAME=picky
ARCH=$(shell ./arch)
VERS=$(shell ../../pick/pick -v| sed 's/.*version GRAPH//g')
DATE=$(shell date)
DIST=1.2.0001

all: $(NAME)_$(ARCH).deb
	mv $(NAME)_*_$(ARCH).deb ../bin

install: all

clean:
	rm -f $(NAME)/bin/* $(NAME)/usr/share/doc/picky/changelog.gz
	rm -f $(NAME)/usr/share/man/


prep:
	echo preparing $(NAME)_$(ARCH)
	mkdir -p $(NAME)/usr/share/man
	mkdir -p $(NAME)/usr/share/man/man1
	cat  ../man/pick.1 |gzip --best > $(NAME)/usr/share/man/man1/picky.1.gz	#pick.1 is taken already by nmh
	cat preinst.$(ARCH) > ./picky/DEBIAN/preinst
	mkdir -p $(NAME)/bin
	cp $$GOBIN/"$$sysos"_"$$mach"/pick $(NAME)/bin/pick
	cp $$GOBIN/"$$sysos"_"$$mach"/pam  $(NAME)/bin/pam
	cat changelog|sed 's/VERS/'$(VERS)'/g'| sed 's/DATE/$(DATE)/g' |gzip --best >  $(NAME)/usr/share/doc/picky/changelog.gz
	

$(NAME)_$(ARCH).deb: prep
	$(eval SIZE=$(shell du -ks . |  sed 's/..$$//g'))
	cat control|sed 's/VERS/'$(VERS)'/g' |sed 's/ARCH/'$(ARCH)'/g' |sed 's/SIZE/'$(SIZE)'/g' 
	cat control|sed 's/VERS/'$(VERS)'/g' |sed 's/ARCH/'$(ARCH)'/g' |sed 's/SIZE/'$(SIZE)'/g' >  $(NAME)/DEBIAN/control
	(cd $(NAME); md5sum `cat ../files` > DEBIAN/md5sums)
	sudo ./debit $(NAME) $(NAME)_1.2.0001-$(VERS)_$(ARCH).deb
	

kosher:
	cd ../bin && lintian picky*.deb && echo the deb files are kosher

.PHONY: all clean kosher install deb prep

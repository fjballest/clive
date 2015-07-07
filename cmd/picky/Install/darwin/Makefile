install: picky.dmg

clean:
	rm -rf picky dmgdir picky.dmg
	
dmgdir/picky.pkg:
	test -d dmgdir || mkdir dmgdir
	test -d picky || mkdir picky
	cp $$GOBIN/"$$sysos"_"$$mach"/pam picky/pam
	cp $$GOBIN/"$$sysos"_"$$mach"/pick picky/pick
	cd picky
	(cd picky; /Applications/PackageMaker.app/Contents/MacOS/PackageMaker --root . --title 'Picky installer' --out ../dmgdir/picky.pkg --scripts ../scripts --id org.lsub.Picky --install-to /bin)
	chmod +x ./dmgdir/picky.pkg
	
picky.dmg: dmgdir/picky.pkg
	rm -f picky.dmg
	hdiutil create -fs HFS+ -srcfolder dmgdir -volname picky picky.dmg

install: picky.dmg
	mv picky.dmg ../bin/picky"$$mach".dmg

.PHONY: all clean install

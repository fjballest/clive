OS="windows linux darwin"

all: insts
	
gocomps:
	./Mkgos

insts:
	mkdir -p bin
	./Mkinst

clean:
	for system in "$(OS)"; do cd $$system; pwd; $(MAKE) clean; cd ..; done
	go env > /tmp/vars.$$
	. /tmp/vars.$$
	rm -f $(GOBIN)/windows*/pick.exe $(GOBIN)/windows*/pam.exe
	rm -f $(GOBIN)/*/pick $(GOBIN)/*/pam
	rm -rf bin



.PHONY: all clean gocomps insts

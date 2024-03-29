GO=go

all: proxid

proxid:
	CGO_ENABLED=0 $(GO) build

install:
	# bin
	install -Dm755 proxid $(DESTDIR)/usr/bin/proxid
	# service
	install -d $(DESTDIR)/usr/lib/systemd/system/
	install -m644 proxid@.service $(DESTDIR)/usr/lib/systemd/system/

clean:
	-@rm -f proxid

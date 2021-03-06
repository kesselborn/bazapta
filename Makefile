VERSION?=$$( git describe --tags || git rev-parse --short HEAD || date "+build%Y%m%d%n" )
GOFLAGS=-v -x -ldflags "-X main.VERSION_STRING $$(echo $(VERSION))"

default: compile

clean:
	rm -rf bin pkg src

# build project using its own GOPATH
PKG=github.com/kesselborn/bazapta
PKG_GOPATH=$(PWD)/src/$(PKG)

build: clean compile

compile: $(PKG_GOPATH)
	GOPATH=$(PWD) go get $(GOFLAGS) -d  ./...
	GOPATH=$(PWD) go install $(GOFLAGS) $(PKG)

$(PKG_GOPATH):
	mkdir -p $$(dirname $(PKG_GOPATH))
	ln -sfn $(PWD) $(PKG_GOPATH)

install: build
	mkdir -p $${DESTDIR-/usr/local}/bin
	cp bin/* $${DESTDIR-/usr/local}/bin

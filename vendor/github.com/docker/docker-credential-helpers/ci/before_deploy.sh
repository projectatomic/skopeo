set -ex

mkdir bin
case "$TRAVIS_OS_NAME" in
	"osx")
		go build -o bin/docker-credential-osxkeychain osxkeychain/cmd/main_darwin.go
		cd bin
		tar czf ../docker-credential-osxkeychain-${TRAVIS_TAG}-amd64.tar.gz docker-credential-osxkeychain
		;;
	"linux")
		go build -o bin/docker-credential-secretservice secretservice/cmd/main_linux.go
		cd bin
		tar czf ../docker-credential-secretservice-${TRAVIS_TAG}-amd64.tar.gz docker-credential-secretservice
		;;
esac

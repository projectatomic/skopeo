{ system ? builtins.currentSystem }:
let
  pkgs = (import ./nixpkgs.nix {
    config = {
      packageOverrides = pkg: {
        gpgme = (static pkg.gpgme);
        libassuan = (static pkg.libassuan);
        libgpgerror = (static pkg.libgpgerror);
      };
    };
  });

  static = pkg: pkg.overrideAttrs(x: {
    doCheck = false;
    configureFlags = (x.configureFlags or []) ++ [
      "--without-shared"
      "--disable-shared"
    ];
    dontDisableStatic = true;
    enableSharedExecutables = false;
    enableStatic = true;
  });

  self = with pkgs; buildGoPackage rec {
    name = "skopeo";
    src = ./..;
    goPackagePath = "github.com/containers/skopeo";
    doCheck = false;
    enableParallelBuilding = true;
    nativeBuildInputs = [ git go-md2man installShellFiles makeWrapper pkg-config ];
    buildInputs = [ glibc glibc.static gpgme libassuan libgpgerror ];
    prePatch = ''
      export CFLAGS='-static'
      export LDFLAGS='-s -w -static-libgcc -static'
      export EXTRA_LDFLAGS='-s -w -linkmode external -extldflags "-static -lm"'
      export BUILDTAGS='static netgo exclude_graphdriver_btrfs exclude_graphdriver_devicemapper'
    '';
    buildPhase = ''
      pushd go/src/${goPackagePath}
      patchShebangs .
      make bin/skopeo
    '';
    installPhase = ''
      install -Dm755 bin/skopeo $out/bin/skopeo
    '';
  };
in self

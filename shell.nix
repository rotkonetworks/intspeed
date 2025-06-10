{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    gnumake
  ];

  shellHook = ''
    echo "🌍 intspeed dev shell"
    export GOROOT="${pkgs.go}/share/go"
    mkdir -p results build
  '';
}

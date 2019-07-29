let
  channels_ = builtins.fromJSON (builtins.readFile ./nixpkgs.json);
  channels  = builtins.mapAttrs (k: v: import (builtins.fetchGit v) {
  }) channels_;
  pkgs = channels."nixos-19.03";
in
  pkgs.mkShell {
    name = "shell";
    buildInputs = [ pkgs.go ];
  }

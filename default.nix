let
  channels_ = builtins.fromJSON (builtins.readFile ./nixpkgs.json);
  channels  = builtins.mapAttrs (k: v: import (builtins.fetchGit v) {
  }) channels_;
  pkgs = channels."nixos-19.03";
in
  pkgs.buildGoModule {
    name = "nomad-driver-systemd";
    src = ./.;
    # modSha256 = pkgs.lib.fakeSha256;
    modSha256 = "00np7sxi22hvn36rgwf8jfl1rkkqazpfhkrgmnryr11wzz2x75xg";
  }

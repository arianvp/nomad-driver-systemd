let
  channels_ = builtins.fromJSON (builtins.readFile ./nixpkgs.json);
  channels  = builtins.mapAttrs (k: v: import (builtins.fetchGit v) {
  }) channels_;
  pkgs = channels."nixpkgs-unstable";
in
  pkgs.buildGoModule {
    name = "nomad-driver-systemd";
    src = ./.;
    # modSha256 = pkgs.lib.fakeSha256;
    modSha256 = "0c2p3315pi19nqirhljq95sl6h4kc6np0dsrzbx417l5nnvflz51";

    # modSha256 = "04c5ck3x8g11d66c9v55n4b0yybm6vfwpxa2p7fiqlniy30hs17c";
    # dunno
    buildInputs = with pkgs; [ nomad vault consul ];
  }

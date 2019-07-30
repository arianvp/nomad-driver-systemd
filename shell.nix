(import ./default.nix).overrideAttrs (_: {
  shellHook = ''
     eval "$configurePhase"
  '';
})

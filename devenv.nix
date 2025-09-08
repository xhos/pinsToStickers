{pkgs,...}: {
  packages = with pkgs; [
    gallery-dl
  ];

  languages.go.enable = true;
}

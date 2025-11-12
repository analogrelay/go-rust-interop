{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  packages = [
    pkgs.cmake
    pkgs.openssl
    pkgs.pkg-config
    pkgs.mold
  ];
  languages.go.enable = true;
  languages.rust.enable = true;
  languages.c.enable = true;
  languages.cplusplus.enable = true;
}

{ lib
, buildGoModule
, nix-gitignore
, installShellFiles
, makeWrapper
, bash
, coreutils
, git
, pandoc
, commit ? null
}:

let
  pname = "gg-scm";
  version = "2.0.0";
in buildGoModule {
  inherit pname version;

  src = let
    root = ./.;
    patterns = nix-gitignore.withGitignoreFile extraIgnores root;
    extraIgnores = [ ".github" ".vscode" "*.nix" "flake.lock" ];
  in builtins.path {
    name = "${pname}-source";
    path = root;
    filter = nix-gitignore.gitignoreFilterPure (_: _: true) patterns root;
  };
  postPatch = ''
    substituteInPlace cmd/gg/editor_unix.go \
      --replace /bin/sh ${bash}/bin/sh
  '';
  subPackages = [ "cmd/gg" ];
  ldflags = [
    "-s" "-w"
    "-X" "main.versionInfo=${version}"
  ] ++ lib.lists.optional (!builtins.isNull commit) [
    "-X" "main.buildCommit=${commit}"
  ];

  vendorHash = "sha256-S8g7BAdGyt4RznioaRO5jmLDMX4O6JZqt1ul3RKAuDQ=";

  nativeBuildInputs = [ pandoc installShellFiles makeWrapper ];
  nativeCheckInputs = [ bash coreutils git ];
  buildInputs = [ bash git ];

  postBuild = ''
    pandoc --standalone --to man misc/gg.1.md -o misc/gg.1
  '';

  postInstall = ''
    wrapProgram $out/bin/gg --suffix PATH : ${git}/bin
    installManPage misc/gg.1
    installShellCompletion --cmd gg \
      --bash misc/gg.bash \
      --zsh misc/_gg.zsh
  '';

  meta = with lib; {
    mainProgram = "gg";
    description = "Git with less typing";
    longDescription = ''
      gg is an alternative command-line interface for Git heavily inspired by Mercurial.
      It's designed for less typing in common workflows,
      making Git easier to use for both novices and advanced users alike.
    '';
    homepage = "https://gg-scm.io/";
    changelog = "https://github.com/gg-scm/gg/blob/v${version}/CHANGELOG.md";
    license = licenses.asl20;
    maintainers = with maintainers; [ zombiezen ];
  };
}

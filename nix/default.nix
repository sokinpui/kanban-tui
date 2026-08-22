{
  buildGoModule,
  lib,
}:
buildGoModule (finalAttrs: {
  pname = "kanban-tui";
  # will need update with new release
  version = "d432016cf6b62b28d2a1b55ef107276d6ec92a92";

  src = ../.;

  # will need update with new release
  vendorHash = "sha256-w4VVf7cAg9dKePIP9gbREV2OJvNJEprfxrBGq+5w1mw=";

  meta = {
    description = "TUI kanban board";
    longDescription = "A terminal-based Kanban board application with a focus on a fast, Vim-like workflow.";
    homepage = "https://github.com/sokinpui/kanban-tui";
    license = lib.licenses.asl20;
    maintainers = with lib.maintainers; [ sokinpui ];
    mainProgram = "kanban";
    platforms = lib.platforms.all;
  };
})

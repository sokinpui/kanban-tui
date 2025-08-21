# kanban

A terminal-based Kanban board application.

## Features

- Vim-like keybindings
- Markdown-based board definition
- Cards as individual markdown files with YAML front matter
- Visual mode for multi-card operations
- Command mode with tab completion for extended functionality
- Undo functionality and command repetition
- Card archiving and a toggleable archive view
- Configurable "Done" column for quick card movement
- Column reordering
- Safe deletion via `trash-cli`

## Dependencies

- Go 1.18+
- [trash-cli](https://github.com/sindresorhus/trash-cli)

## Installation

Ensure `trash-cli` is installed and available in your `PATH`.

```sh
# Example using npm
npm install --global trash-cli
```

Install the application using `go`:

```sh
go install github.com/sokinpui/kanban/cmd/kanban@latest
```

## Usage

Run `kanban` in any directory.

If `kanban.md` is not found, the application will offer to create a sample board structure.

## File Structure

The application operates on a simple file-based structure.

- `kanban.md`: The main board file. Columns are defined by level-1 markdown headers (`#`). Cards are represented as list items linking to their respective files.

  ```markdown
  # To Do

  - [Refactor networking layer](./.kanban/To Do/f1b3e3a3-....md)

  # In Progress

  - [Implement UI components](./.kanban/In Progress/a2c4e5b5-....md)
  ```

- `.kanban/`: A hidden directory containing all application data.
  - `.kanban/{Column Name}/`: A subdirectory for each column.
  - `.kanban/{Column Name}/{UUID}.md`: The markdown file for each card. Contains YAML front matter for metadata and markdown for content.
  - `.kanban/Archived/`: A special directory for archived cards.
  - `.kanban/state.json`: Persists the last focused view state, including the name of the 'Done' column and archive visibility.

## Keybindings

### Normal Mode

| Key          | Action                              |
| ------------ | ----------------------------------- |
| `q`, `C-c`   | Quit                                |
| `h`, `left`  | Focus column to the left            |
| `l`, `right` | Focus column to the right           |
| `k`, `up`    | Focus card above, or column header  |
| `j`, `down`  | Focus card below                    |
| `gg`         | Focus first card in column          |
| `G`          | Focus last card in column           |
| `enter`      | Open focused card in `$EDITOR`      |
| `o`          | Create new card after focused card  |
| `O`          | Create new card before focused card |
| `yy`         | Yank (copy) focused card            |
| `dd`         | Cut focused card                    |
| `p`          | Paste after focused position        |
| `P`          | Paste before focused position       |
| `v`, `V`     | Enter visual mode                   |
| `delete`     | Delete focused card                 |
| `u`          | Undo last action                    |
| `.`          | Repeat last command                 |
| `:`          | Enter command mode                  |
| `esc`        | Clear selection and clipboard       |

### Visual Mode

| Key          | Action                             |
| ------------ | ---------------------------------- |
| `esc`, `v`   | Exit visual mode                   |
| `j`, `down`  | Extend selection down              |
| `k`, `up`    | Extend selection up                |
| `gg`         | Extend selection to the first card |
| `G`          | Extend selection to the last card  |
| `y`          | Yank (copy) selected cards         |
| `d`          | Cut selected cards                 |
| `delete`     | Delete selected cards              |
| `h`, `left`  | Exit visual mode                   |
| `l`, `right` | Exit visual mode                   |
| `:`          | Enter command mode                 |

### Command Mode

| Key          | Action               |
| ------------ | -------------------- |
| `enter`      | Execute command      |
| `esc`, `C-c` | Exit command mode    |
| `tab`        | Autocomplete command |

## Commands

Commands are entered after pressing `:`. Tab completion is available.

### Card Management

- `:new {title}`
  Create a new card in the focused column.

- `:done`
  Move selected/focused card(s) to the configured 'Done' column.

- `:archive`
  Archive selected cards. Cards are moved to a special 'Archived' column.

### Column Management

- `:create {name}`
  Create a new column.

- `:delete`
  Delete the focused column (only when the header is focused).

- `:left`
  Move focused column to the left.

- `:right`
  Move focused column to the right.

- `:sort {field} {direction}`
  Sort cards in the focused column.
  - `field`: `create` or `modify`
  - `direction`: `asc` or `desc`

### Application & View Settings

- `:set done`
  Set the focused column as the 'Done' column.

- `:set done?`
  Show the name of the current 'Done' column.

- `:unset done`
  Clear the 'Done' column setting.

- `:show hidden`
  Show the 'Archived' column on the board.

- `:hide hidden`
  Hide the 'Archived' column from the board.

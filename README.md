# kanban

A terminal-based Kanban board application.

## Features

- Vim-like keybindings
- Markdown-based board definition
- Cards as individual markdown files with YAML front matter
- Visual mode for multi-card operations
- Command mode for extended functionality
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
go install github.com/your-username/kanban/cmd/kanban@latest
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
  - `.kanban/state.json`: Persists the last focused view state.

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
| `:`          | Enter command mode                  |
| `esc`        | Clear selection and clipboard       |

### Visual Mode

| Key         | Action                             |
| ----------- | ---------------------------------- |
| `esc`, `v`  | Exit visual mode                   |
| `j`, `down` | Extend selection down              |
| `k`, `up`   | Extend selection up                |
| `gg`        | Extend selection to the first card |
| `G`         | Extend selection to the last card  |
| `y`         | Yank (copy) selected cards         |
| `d`         | Cut selected cards                 |
| `delete`    | Delete selected cards              |

### Command Mode

| Key          | Action            |
| ------------ | ----------------- |
| `enter`      | Execute command   |
| `esc`, `C-c` | Exit command mode |

## Commands

Commands are entered after pressing `:`.

- `:new {title}`
  Create a new card.

- `:create {name}`
  Create a new column.

- `:delete`
  Delete the focused column. This is only possible when the column header is focused (no card is selected).

- `:sort {field} {direction}`
  Sort cards in the focused column.
  - `field`: `create` or `modify`
  - `direction`: `asc` or `desc`

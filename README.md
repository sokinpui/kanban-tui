# kanban

A terminal-based Kanban board application with a focus on a fast, Vim-like workflow.

## Features

- Modern, informative status bar with mode indicators and progress
- Vim-like keybindings for all major operations
- Fuzzy finder for quick card navigation (`fzf`-like)
- Forward and backward search for cards (`/`, `?`)
- Markdown-based board definition
- Cards as individual markdown files with YAML front matter
- Visual mode for multi-card operations
- Command mode with tab completion for extended functionality
- Undo/redo functionality and command repetition
- Card archiving and a toggleable archive view
- Configurable "Done" column for quick card movement
- Column creation, deletion, renaming, and reordering
- Safe deletion via `trash-cli`
- **Nested/Linked Boards**: Create cards that link to other `kanban.md` files and navigate between them seamlessly.
- **Meta-Boards**: Aggregate multiple project boards into a single master view for high-level tracking.

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
go install github.com/sokinpui/kanban-tui/cmd/kanban@latest
```

## Usage

Run `kanban` in any directory.

If `kanban.md` is not found, the application will offer to create a sample board structure.

### Meta-Boards (Master View)

You can create a "meta-board" that aggregates cards from multiple other kanban projects. This is useful for a high-level overview.

Run the following command in a directory where you want to create your master board:

```sh
kanban --main path/to/project1/kanban.md path/to/project2/kanban.md
```

This will create a board with `buffer` and `projects` columns and add cards that link to your specified project boards.

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
| `q`, `C-c`   | Quit, or return to parent board     |
| `h`, `left`  | Focus column to the left            |
| `l`, `right` | Focus column to the right           |
| `k`, `up`    | Focus card above, or column header  |
| `j`, `down`  | Focus card below                    |
| `gg`         | Focus first card in column          |
| `G`          | Focus last card in column           |
| `/`          | Enter forward search mode           |
| `?`          | Enter backward search mode          |
| `gf`         | Go to file (follow link to board)   |
| `n`          | Find next search result             |
| `N`          | Find previous search result         |
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
| `C-r`        | Redo last undone action             |
| `.`          | Repeat last command                 |
| `:`          | Enter command mode                  |
| `C-p`        | Open fuzzy finder                   |
| `gf`         | Go to file (follow link in card)    |
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

### Search Mode

| Key          | Action                               |
| ------------ | ------------------------------------ |
| `enter`      | Execute search and return to normal  |
| `esc`, `C-c` | Cancel search and return to normal   |

### Command Mode

| Key          | Action               |
| ------------ | -------------------- |
| `enter`      | Execute command      |
| `esc`, `C-c` | Exit command mode    |
| `tab`        | Autocomplete command |

## Commands

Commands are entered after pressing `:`. Tab completion is available.

### Navigation & Search

- `:fzf`
  Open the fuzzy finder to search for cards.

- `:noh`, `:nohlsearch`
  Clear the last search term (stops `n`/`N` from working).

### Card & Column Management

- `:new {title}`
  Create a new card in the focused column.

- `:done`
  Move selected/focused card(s) to the configured 'Done' column.

- `:archive`
  Archive selected cards. Cards are moved to a special 'Archived' column.

- `:create {name}`
  Create a new column.

- `:delete`
  Delete the focused column (only when the header is focused).

- `:rename {new-name}`
  Rename the focused column.

- `:left`
  Move focused column to the left.

- `:right`
  Move focused column to the right.

- `:sort {field}[!`]`
  Sort cards in the focused column.
  - `field`: `name` (default), `create`, `modify`, or `size`.
  - `!`: Add `!` to the end of the command for descending order (e.g., `:sort create!`).

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

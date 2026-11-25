## ADDED Requirements

### Requirement: musica is the player command

The tesseract binary SHALL answer as `musica`. It MUST play a track, open a directory for one run, edit config, serve the player to the Tesseract Musica center, and end the running session. The saved library MUST open in the Tesseract Musica center, the third center of the Super+T launcher. Bare `musica` with no arguments MUST run `<config>/quickshell/omarchy-launcher/open.sh musica`, `<config>` being `$XDG_CONFIG_HOME` when it is absolute and `~/.config` otherwise, and MUST NOT exec `r2d2`. `musica serve`, `musica end`, `musica reload`, `musica download-worker`, `musica play`, `musica open`, `musica config`, `musica help`, `musica version`, and `musica upgrade` MUST stay in the musica binary. `musica play` and `musica open` MUST still open the terminal TUI. Help MUST say the player opens in the Tesseract Musica center with `musica` or `r2d2 musica`. Config MUST live at `$XDG_CONFIG_HOME/r2d2/musica/config.json`, or `~/.config/r2d2/musica/config.json` when `XDG_CONFIG_HOME` is unset, and musica MUST write only that path. When that file is absent, musica MUST copy `<config>/musica/config.json` once, or else `<config>/rlc/config.json` once, before it reads or writes config, and MUST leave the old file in place. `musica end` MUST return only once the session has let go of its lock, so that a `musica serve` started next holds the session at once, and MUST fail with `musica did not end` when the session still holds it after five seconds. Before it sends `end`, `musica end` MUST note the track the session's ffplay has loaded, playing or paused, and MUST read it from the session's ffplay process, because a terminal session from an older musica answers only `end`; with no ffplay loaded it MUST forget an older note. The next `musica serve` MUST take that note and open the track's cassette with the track selected and the cassette menu open, its frames naming the track with the state `paused` and no elapsed time. It MUST NOT start ffplay for that track and MUST NOT restart it from the beginning on its own: Space MUST say there is nothing to resume, Enter on the track MUST play it from the start, and a stop MUST put the held track down.

#### Scenario: Launch the saved library

- **GIVEN** a saved `music_dir`
- **WHEN** the operator runs `musica` or `r2d2 musica` with no further arguments
- **THEN** the Tesseract Musica center MUST show that library
- **AND** playback MUST continue when the operator leaves the center

#### Scenario: Bare musica opens the Tesseract center

- **WHEN** the operator runs `musica` with no arguments
- **THEN** musica MUST run `~/.config/quickshell/omarchy-launcher/open.sh musica`
- **AND** it MUST NOT exec `r2d2`
- **AND** the environment MUST pass through unchanged

#### Scenario: Adopt the musica config

- **GIVEN** `~/.config/musica/config.json` exists and `~/.config/r2d2/musica/config.json` does not
- **WHEN** musica loads config
- **THEN** it MUST copy the musica config to `~/.config/r2d2/musica/config.json`
- **AND** `~/.config/musica/config.json` MUST stay in place
- **AND** later saves MUST write only `~/.config/r2d2/musica/config.json`

#### Scenario: Adopt the previous config

- **GIVEN** `~/.config/rlc/config.json` exists and neither `~/.config/musica/config.json` nor `~/.config/r2d2/musica/config.json` does
- **WHEN** musica loads config
- **THEN** it MUST copy the rlc config to `~/.config/r2d2/musica/config.json`

#### Scenario: The new config wins

- **GIVEN** `~/.config/r2d2/musica/config.json` exists
- **WHEN** musica loads config
- **THEN** it MUST NOT copy an old file over it

#### Scenario: musica end waits for the session to let go

- **GIVEN** a session holds the musica control socket
- **WHEN** the operator runs `musica end`
- **THEN** it MUST return only after the session has let go of its lock
- **AND** a `musica serve` started next MUST hold the session

#### Scenario: A paused terminal track opens paused in musica serve

- **GIVEN** a terminal TUI holds the session with `focus/Flow.mp3` loaded and paused
- **WHEN** `musica end` ends it and `musica serve` starts
- **THEN** the first frame MUST show the `focus` cassette with `Flow` selected, the title `Flow`, and the state `paused`
- **AND** no ffplay MUST start
- **AND** Space MUST NOT start `Flow.mp3`

### Requirement: nebula s3 backs up application state

musica MUST keep downloads and the saved music directory in its own sqlite file. While the machine is up and `SQLX_ROOT` is unset, that file MUST be `$XDG_RUNTIME_DIR/musica/musica.sqlite`. nebula `s3` MUST store the backup at `s3://backups/musica.sqlite`. The application MUST NOT call nebula sql. A failed s3 call MUST NOT roll back the local sqlite write. Audio files MUST remain in the configured music directory.

#### Scenario: Restore a missing database

- **GIVEN** the runtime sqlite file is missing and `s3://backups/musica.sqlite` exists
- **WHEN** musica opens its database
- **THEN** it MUST restore that object before creating a new file

#### Scenario: Publish after a download changes

- **GIVEN** a download is queued, finished, failed, or cancelled
- **WHEN** musica records that state
- **THEN** it MUST put the sqlite file at `s3://backups/musica.sqlite`

### Requirement: YouTube downloads continue outside the TUI

From an open cassette, `y` MUST ask for a YouTube URL and then a song name. The download MUST be queued and shown in a download view whose running progress animation changes over time. The download process MUST keep running after the TUI exits and while another track is playing. Cancel from the download view MUST stop that download and MUST NOT stop other downloads or playback. `u` MUST open the download view. A download that has finished MUST leave the download view. A queued, running, failed, or cancelled download MUST stay visible. The finished audio MUST remain in the cassette. `j` and `k` MUST wrap the rows in that view. On the last row, `j` MUST move to the first row. On the first row, `k` MUST move to the last row. An empty view MUST leave the cursor in place.

#### Scenario: Queue a download

- **GIVEN** a cassette is open
- **WHEN** the operator submits a YouTube URL and a song name
- **THEN** musica MUST queue the download and show the download view
- **AND** the operator MUST be able to return to the library and play another track while it runs

#### Scenario: Close the app during a download

- **GIVEN** a download is running
- **WHEN** the operator closes the TUI
- **THEN** the download worker MUST keep running
- **AND** opening musica again MUST show that download's progress

#### Scenario: Cancel from the download view

- **GIVEN** the download view has a selected running download and music is playing
- **WHEN** the operator cancels that download
- **THEN** that download MUST stop
- **AND** playback MUST continue

#### Scenario: A finished download leaves the view

- **GIVEN** one download has finished and another is running
- **WHEN** the operator opens the download view
- **THEN** the finished download MUST be absent
- **AND** the running download MUST remain

#### Scenario: Completion removes the row

- **GIVEN** a download is running in the download view
- **WHEN** that download finishes
- **THEN** its row MUST leave the download view
- **AND** the audio MUST remain in the cassette

#### Scenario: j and k wrap the download rows

- **GIVEN** the download view lists two downloads and the cursor is on the last
- **WHEN** the operator presses `j`
- **THEN** the cursor MUST be on the first download
- **AND** `k` MUST move it back to the last download

### Requirement: Pause opens the cassette menu

Space MUST pause or resume the current track. Pausing MUST open the cassette menu and MUST leave the download view. In the terminal player, resuming MUST leave the menu as it is. In the Tesseract center, resuming MUST close the cassette menu, and the frame MUST show state `playing` with the menu closed, so the picture returns. Space MUST NOT cancel a download. When nothing is playing, space MUST NOT open the menu.

#### Scenario: Pause while the library is showing

- **GIVEN** a track is playing and the cassette menu is closed
- **WHEN** the operator presses space
- **THEN** playback MUST pause
- **AND** the cassette menu MUST open

#### Scenario: Pause from the download view

- **GIVEN** a track is playing and the download view is open
- **WHEN** the operator presses space
- **THEN** playback MUST pause
- **AND** the cassette menu MUST open
- **AND** the download MUST keep running

#### Scenario: Resume leaves the menu in the terminal player

- **GIVEN** playback is paused and the cassette menu is open in the terminal player
- **WHEN** the operator presses space
- **THEN** playback MUST resume
- **AND** the cassette menu MUST stay open

#### Scenario: Resume brings the picture back in the center

- **GIVEN** the Tesseract center has a track paused and the cassette menu open
- **WHEN** the operator presses space
- **THEN** playback MUST resume
- **AND** the cassette menu MUST close
- **AND** the frame state MUST be `playing`

### Requirement: The player text is white

The visualizer glyphs, status line, help, download view, and YouTube prompt MUST use white text. They MUST NOT use a separate hue for titles, status, errors, selection, playback, or download state. Visualizer rows MUST share one base color. Empty visualizer cells MAY stay dark.

#### Scenario: Text is white

- **GIVEN** the player is drawing
- **WHEN** a title, a status line, a rain glyph, a running download, and a failed download are styled
- **THEN** each foreground MUST be white
- **AND** the running download and the failed download MUST share that white

#### Scenario: Visualizer rows share one color

- **GIVEN** the visualizer is drawing
- **WHEN** the top, middle, and bottom rows are styled
- **THEN** those rows MUST use the same base color

### Requirement: q ends the session

`q` MUST stop playback and end the terminal TUI when no text prompt is open, including from the library, the cassette menu, help, and the download view. `q` MUST NOT end `musica serve`, stop its playback, or send `ended`. `q` MUST NOT cancel a background download. While a text prompt is open, `q` MUST be typed and MUST NOT quit.

#### Scenario: q closes a playing terminal session

- **GIVEN** a track is playing in the TUI, and no text prompt is open
- **WHEN** the operator presses `q`
- **THEN** playback MUST stop
- **AND** the TUI MUST quit

#### Scenario: q does not end musica serve

- **GIVEN** a track is playing in `musica serve`, and no text prompt is open
- **WHEN** the operator presses `q`
- **THEN** playback MUST continue
- **AND** the session MUST stay up and MUST NOT send `ended`

#### Scenario: q from the download view leaves the download running

- **GIVEN** a track is playing and the download view shows a running download
- **WHEN** the operator presses `q`
- **THEN** the session MUST end
- **AND** that download MUST keep running

#### Scenario: q types into a prompt

- **GIVEN** a text prompt is open
- **WHEN** the operator presses `q`
- **THEN** the prompt MUST contain `q`
- **AND** the session MUST stay open

### Requirement: r2d2 opens the player

The module MUST provide the public package `github.com/ryangerardwilson/tesseract/player` with `Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int`. `Run` MUST run the same player command as the musica binary: with no arguments it MUST open the Tesseract Musica center as bare `musica` does, and every other argument list MUST behave as it does on the binary. `Run` MUST NOT exec `r2d2`. The player implementation MUST stay in this module.

#### Scenario: r2d2 opens the library

- **WHEN** r2d2 calls `player.Run` with no arguments
- **THEN** the launcher's `open.sh musica` MUST run
- **AND** the Tesseract Musica center MUST show the saved library

#### Scenario: r2d2 ends the session

- **WHEN** r2d2 calls `player.Run` with `end`
- **THEN** the running musica session MUST end

### Requirement: The download worker runs on the musica binary

The player MUST start the download worker by execing `musica` found on PATH with the argument `download-worker`, in a new session. It MUST NOT exec the current executable. When `musica` is not on PATH, starting the worker MUST fail with that error. `musica download-worker` MUST drain the queue.

#### Scenario: The player runs inside r2d2

- **GIVEN** the player runs inside r2d2
- **WHEN** a download is queued
- **THEN** the worker MUST be `musica download-worker`
- **AND** it MUST NOT be `r2d2 download-worker`

#### Scenario: musica is missing

- **GIVEN** `musica` is not on PATH
- **WHEN** the player starts the worker
- **THEN** starting MUST fail with the lookup error

### Requirement: musica serve streams the player to the Tesseract center

`musica serve` SHALL run the player with no terminal. It SHALL hold the musica session and its control socket as the TUI does, play through ffplay, keep playing after every client disconnects, and end only on Ctrl+C, `musica end`, SIGTERM, SIGINT, or SIGHUP, stopping playback first. `q` SHALL NOT end it. `reload` SHALL replace the process with the binary on disk, continue the loaded track at its offset, paused when it was paused, and MUST NOT send `ended`. A client that writes `watch` and a newline SHALL receive one JSON object per line: a `frame` after each player update, holding the playing state, elapsed time, and title, the status line, the view, the open cassette, up to nine rows around the selection, the open prompt and its text, help, the download view with its rows and selection, and the visualizer's 32 band `levels` and `peaks` with `bass`, `energy`, and `thump`, each from 0 to 1, `thump` being the hardest low-end hit the analyzer heard since the frame before, so a held tone hits only as it starts; a `leave` event; and an `ended` event when the session ends. Each line the client writes after `watch` SHALL be a JSON object `{"key":"<name>"}` that the player takes as that key with the TUI's rules, except that `m` with no prompt open SHALL toggle the cassette menu and SHALL NOT send `leave`, and Escape from anywhere SHALL send `leave` and leave playback, the menu, and the selection as they were. Escape SHALL back out of nothing in `musica serve`: it SHALL NOT cancel a prompt, close help or the download view, or go back from a cassette, which stay for the next showing. `h` with no prompt or leader open SHALL go back to the cassette list from an open cassette's tracks, help, or the download view, showing the menu with that cassette selected, and on the cassette list SHALL move the selection as in the TUI. The terminal TUI SHALL keep Escape as back. A client that cannot keep up SHALL be dropped and MUST NOT stall the player. No frame SHALL be built while no client watches. `end` SHALL still end the session. The TUI session SHALL answer `watch` with `unknown command`.

#### Scenario: A watcher gets frames

- **GIVEN** `musica serve` is playing a track
- **WHEN** a client writes `watch`
- **THEN** it MUST receive `frame` lines whose `levels` and `peaks` hold 32 numbers from 0 to 1
- **AND** each frame MUST name the playing track

#### Scenario: A kick reaches one frame

- **GIVEN** a watcher and a track playing
- **WHEN** a kick lands in one analysis window and quiet windows follow
- **THEN** the next frame's `thump` MUST be near 1
- **AND** the frames after it MUST carry a `thump` near 0

#### Scenario: A held tone does not thump

- **GIVEN** a watcher and a track that holds one low tone
- **WHEN** frames follow the tone's start
- **THEN** their `thump` MUST stay near 0

#### Scenario: A key line drives the player

- **GIVEN** a watcher and the cassette list showing its last row selected
- **WHEN** the client writes `{"key":"j"}`
- **THEN** a later frame MUST show the first row selected

#### Scenario: Space pauses and opens the cassette actions

- **GIVEN** a watcher and a track playing with the cassette menu closed
- **WHEN** the client writes `{"key":" "}`, and then `{"key":" "}` again
- **THEN** playback MUST pause and a frame MUST show the menu open
- **AND** the second space MUST resume playback with the menu still open

#### Scenario: m toggles the cassette menu

- **GIVEN** a watcher, a track playing, the cassette menu closed, and no prompt open
- **WHEN** the client writes `{"key":"m"}`, and then `{"key":"m"}` again
- **THEN** a frame MUST show the menu open, and the next `m` MUST show it closed
- **AND** the client MUST NOT receive `leave`
- **AND** playback MUST continue

#### Scenario: Escape leaves from anywhere and backs out of nothing

- **GIVEN** a watcher and an open cassette, help, the download view, or a prompt
- **WHEN** the client writes `{"key":"esc"}`
- **THEN** the client MUST receive `leave`
- **AND** a later frame MUST still show the open cassette, help, the download view, or the prompt and its text

#### Scenario: h goes back to the cassette list

- **GIVEN** a watcher and an open cassette's tracks, help, or the download view, with no prompt open
- **WHEN** the client writes `{"key":"h"}`
- **THEN** a later frame MUST show the cassette list with the menu open and that cassette selected
- **AND** the client MUST NOT receive `leave`

#### Scenario: Playback outlives the watcher

- **GIVEN** `musica serve` is playing a track for a watcher
- **WHEN** the watcher disconnects
- **THEN** playback MUST continue and the session MUST stay up

#### Scenario: reload continues on the binary on disk

- **GIVEN** `musica serve` is playing a track 90 seconds in
- **WHEN** a client writes `reload`
- **THEN** the process MUST be replaced by the binary on disk
- **AND** playback MUST continue from that offset
- **AND** the session MUST NOT send `ended`

#### Scenario: The TUI refuses watch

- **GIVEN** the terminal TUI holds the session
- **WHEN** a client writes `watch`
- **THEN** it MUST read `unknown command`

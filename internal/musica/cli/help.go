package cli

const Usage = "Use: musica | r2d2 musica | musica serve | musica play <track> | musica open <music_dir> | musica config | musica end | musica reload"

const HelpText = `musica terminal music player

global actions:
  musica help
    show this help
  musica version
    print the installed version
  musica upgrade
    upgrade to the latest release
  musica end
    stop playback, end the running musica session, and wait until it lets go;
    the next musica serve opens the cassette it had loaded, paused
  musica reload
    replace the running session with this binary and continue playback where it was

features:
  show your saved library in the Tesseract Musica center, the third center of Super+T
  # musica | r2d2 musica
  musica
  r2d2 musica

  run the player for that center with no terminal; the launcher starts it
  # serve
  musica serve

  play a specific track immediately in the terminal player
  # play <track>
  musica play ~/Music/song.mp3

  use a directory for one run of the terminal player without changing saved config
  # open <music_dir>
  musica open ~/Music

  open the user config in your editor
  # config
  musica config
`

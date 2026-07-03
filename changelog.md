# BitBeat Project Changelog

## Added saved entries feature with persistent storage
- Implemented TUI menu system with Saved Entries, Link, and Exit options
- Added ability to name and save links for quick access
- Created persistent storage using JSON file (saved_entries.json)
- Enhanced UI with three-screen workflow: main menu → saved entries list → add entry form
- Maintained backward compatibility with existing Link functionality
- All navigation and exit controls work as expected (q to quit, ESC to go back)

## b17f1b9 | 2026-06-08 12:28:11 +0530 | MalG4850
- Made BitBeat with SoundCloud Support.
  - Initial project creation
  - Added core audio playback functionality
  - Integrated SoundCloud API support
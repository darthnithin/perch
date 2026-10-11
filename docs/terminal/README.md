# Perch terminal profile

The approved macOS reference is **Ioskeley Mono, 12 pt, 0.90 line spacing**.
The user reviewed this profile and accepted its appearance on October 10, 2026.
That acceptance covers this reference setup; it does not certify every font,
terminal or application state.

1. Install Ioskeley Mono's regular and bold fonts through Font Book, if needed.
2. Download [Perch-Ioskeley.terminal](Perch-Ioskeley.terminal). On GitHub, use the
   file's **Download raw file** button rather than saving the HTML page.
3. Double-click the file to import/open it in Terminal.app. It opens a normal shell.
4. From the source checkout, run `./bin/perch --demo --no-config` or connect with
   `./bin/perch --host your-cruzid@hb.ucsc.edu`.

The profile has a warm charcoal background, cream text, peach cursor, normal
character width, and a 140-column × 44-row starting window. It contains no
startup command, machine-specific path or SSH destination. Importing it does
not select it as your default; that is your choice in Terminal settings.

Fonts are not bundled. Chip uses Unicode 16 octants, present in the installed
Ioskeley version used for review. Missing glyphs, line gaps and fallback shapes
in other fonts are controlled by the font and terminal renderer. Keep the
approved artwork; do not stretch it to compensate for a different font.

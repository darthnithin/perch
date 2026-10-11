# Perch terminal profile

The approved macOS reference is **Ioskeley Mono, 12 pt, 0.90 line spacing**.
The user reviewed this profile and accepted its appearance on October 10, 2026.
That acceptance covers this reference setup; it does not certify every font,
terminal or application state.

1. Download the standard `IoskeleyMono.zip` from the
   [font project’s releases](https://github.com/ahatem/IoskeleyMono/releases),
   then install its regular and bold fonts through Font Book. This profile uses
   the standard **Ioskeley Mono** family, not the separately named Term variant.
2. Download [Perch-Ioskeley.terminal](Perch-Ioskeley.terminal). On GitHub, use the
   file's **Download raw file** button rather than saving the HTML page.
3. Double-click the file to import/open it in Terminal.app. It opens a normal shell.
4. From the extracted preview folder, run `./perch --demo --no-config` or connect
   with `./perch --host your-cruzid@hb.ucsc.edu`. From a built source checkout,
   use `./bin/perch` instead. If Perch is installed on your PATH, use `perch`.

The profile has a warm charcoal background, cream text, peach cursor, normal
character width, and a 140-column × 44-row starting window. It contains no
startup command, machine-specific path or SSH destination. Importing it does
not select it as your default; that is your choice in Terminal settings.

Fonts are not bundled. Chip uses Unicode 16 octants, present in the installed
Ioskeley version used for review. Missing glyphs, line gaps and fallback shapes
in other fonts are controlled by the font and terminal renderer. Keep the
approved artwork; do not stretch it to compensate for a different font.

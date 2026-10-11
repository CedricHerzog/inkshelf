#!/bin/sh
# Deploys the app to a PocketBook reader mounted over USB ("PC link" on the reader):
# copies the app and its launcher icons, creates the app's config folder, then ejects.
#
# The reader's volume is detected under /Volumes; set PB_VOLUME to override.
set -eu

# The reader's storage has lowercase applications/ and system/ folders.
# Compare names exactly: macOS paths are case-insensitive, so a plain -d test
# would also match the boot disk's /Applications and /System (and
# /Volumes/Macintosh HD is a symlink to /, hence skipping symlinks too).
find_volume() {
	for v in /Volumes/*; do
		[ -L "$v" ] && continue
		entries=$(ls -1 "$v" 2>/dev/null) || continue
		if printf '%s\n' "$entries" | grep -qx applications &&
			printf '%s\n' "$entries" | grep -qx system; then
			echo "$v"
			return 0
		fi
	done
	return 1
}

vol=${PB_VOLUME:-$(find_volume || true)}
if [ -z "$vol" ]; then
	echo "No PocketBook found under /Volumes. Connect the reader over USB and choose 'PC link'." >&2
	exit 1
fi

[ -f build/inkshelf.app ] || { echo "build/inkshelf.app is missing; run 'make build' first." >&2; exit 1; }
# The app was called komga: its folder (config.json, downloads, filters) becomes inkshelf's,
# and its program, icons and log go.
if [ -d "$vol/applications/komga" ] && [ ! -e "$vol/applications/inkshelf" ]; then
	mv "$vol/applications/komga" "$vol/applications/inkshelf"
	echo "Moved applications/komga/ to applications/inkshelf/"
fi
rm -f "$vol/applications/komga.app" "$vol/applications/komga.log" \
	"$vol/applications/icons/komga.bmp" "$vol/applications/icons/komga_f.bmp"
# applications/inkshelf/ holds config.json (servers and API keys);
# applications/icons/ the launcher icons that view.json points to.
mkdir -p "$vol/applications/inkshelf" "$vol/applications/icons"
# -X and COPYFILE_DISABLE keep macOS from writing ._* files, which the
# reader's launcher would list as broken apps.
export COPYFILE_DISABLE=1
cp -X build/inkshelf.app "$vol/applications/"
# A copy once came out corrupted (same size, different bytes): check before ejecting.
sync
cmp -s build/inkshelf.app "$vol/applications/inkshelf.app" ||
	{ echo "applications/inkshelf.app differs from build/inkshelf.app after copying; run deploy again." >&2; exit 1; }
cp -X icons/inkshelf.bmp icons/inkshelf_f.bmp "$vol/applications/icons/"
rm -f "$vol/applications/._inkshelf.app" "$vol/applications/._icons" \
	"$vol/applications/icons/._inkshelf.bmp" "$vol/applications/icons/._inkshelf_f.bmp"
sync
echo "Deployed to $vol"
# The launcher entry is edited by hand (docs/reference/usage.md): say so if it still starts the old app.
if grep -q 'applications/komga\.app' "$vol/system/config/desktop/view.json" 2>/dev/null; then
	echo "system/config/desktop/view.json still starts applications/komga.app: rename komga to inkshelf in its" >&2
	echo "entry (U_komga, its path and icons), as docs/reference/usage.md says." >&2
fi
diskutil eject "$vol"

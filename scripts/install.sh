#!/bin/sh
# Install the complete built bundle, then expose its executable on PATH.
set -eu

prefix=${1:-"$HOME/.local"}
case "$prefix" in
  /*) ;;
  *) echo "Installation prefix must be an absolute path." >&2; exit 2 ;;
esac
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
distribution="$script_dir/../dist"
destination="$prefix/share/patchbot"
command_path="$prefix/bin/patchbot"

if [ ! -x "$distribution/patchbot" ] ||
   [ ! -f "$distribution/analyzer/src/index.mjs" ] ||
   [ ! -f "$distribution/analyzer/node_modules/@npmcli/arborist/package.json" ] ||
   [ ! -f "$distribution/analyzer/node_modules/ts-morph/package.json" ]; then
  echo "Complete distribution missing. Run make build first." >&2
  exit 2
fi
if [ -e "$command_path" ] || [ -L "$command_path" ]; then
  if [ ! -L "$command_path" ] || [ "$(readlink "$command_path")" != "$destination/patchbot" ]; then
    echo "Refusing to replace an unrelated command: $command_path" >&2
    exit 2
  fi
fi
if [ -L "$destination" ] || { [ -e "$destination" ] && [ ! -f "$destination/.patchbot-install" ]; }; then
  echo "Refusing to replace an unrecognized installation: $destination" >&2
  exit 2
fi

mkdir -p "$prefix/bin" "$destination/analyzer"
cp -R "$distribution/analyzer/." "$destination/analyzer/"
install -m 755 "$distribution/patchbot" "$destination/patchbot"
printf '%s\n' 'PatchBot user installation' > "$destination/.patchbot-install"
ln -sf "$destination/patchbot" "$command_path"
printf 'Installed %s\n' "$command_path"
case ":$PATH:" in
  *":$prefix/bin:"*) ;;
  *) printf 'Add this directory to your shell PATH: %s/bin\n' "$prefix" ;;
esac

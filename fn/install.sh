#!/bin/sh
set -e

# Install script to install fn

release="0.2.57"

command_exists() {
  command -v "$@" > /dev/null 2>&1
}

case "$(uname -m)" in
  *64)
    ;;
  *)
    echo >&2 'Error: you are not using a 64bit platform.'
    echo >&2 'Functions CLI currently only supports 64bit platforms.'
    exit 1
    ;;
esac

if command_exists fn ; then
  echo >&2 'Warning: "fn" command appears to already exist.'
  echo >&2 'If you are just upgrading your functions cli client, ignore this and wait a few seconds.'
  echo >&2 'You may press Ctrl+C now to abort this process.'
  ( set -x; sleep 5 )
fi

user="$(id -un 2>/dev/null || true)"

sh_c='sh -c'
if [ "$user" != 'root' ]; then
  if command_exists sudo; then
    sh_c='sudo -E sh -c'
  elif command_exists su; then
    sh_c='su -c'
  else
    echo >&2 'Error: this installer needs the ability to run commands as root.'
    echo >&2 'We are unable to find either "sudo" or "su" available to make this happen.'
    exit 1
  fi
fi

curl=''
if command_exists curl; then
  curl='curl -sSL -o'
elif command_exists wget; then
  curl='wget -qO'
elif command_exists busybox && busybox --list-modules | grep -q wget; then
  curl='busybox wget -qO'
else
    echo >&2 'Error: this installer needs the ability to run wget or curl.'
    echo >&2 'We are unable to find either "wget" or "curl" available to make this happen.'
    exit 1
fi

url='https://github.com/iron-io/functions/releases/download'

# perform some very rudimentary platform detection
case "$(uname)" in
  Linux)
    $sh_c "$curl /usr/local/bin/fn $url/$release/fn_linux"
    $sh_c "chmod +x /usr/local/bin/fn"
    fn --version
    exit 0
    ;;
  Darwin)
    $sh_c "$curl /usr/local/bin/fn $url/$release/fn_mac"
    $sh_c "chmod +x /usr/local/bin/fn"
    fn --version
    exit 0
    ;;
  WindowsNT)
    $sh_c "$curl $url/$release/fn.exe"
    # TODO how to make executable? chmod?
    fn.exe --version
    exit 0
    ;;
esac

cat >&2 <<'EOF'

  Either your platform is not easily detectable or is not supported by this
  installer script (yet - PRs welcome! [fn/install]).
  Please visit the following URL for more detailed installation instructions:

    https://github.com/iron-io/functions

EOF
exit 1

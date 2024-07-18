# https://download.mozilla.org/?product=firefox-devedition-latest-ssl&os=linux64&lang=en-US
curl --location
"https://download.mozilla.org/?product=firefox-devedition-latest-ssl&os=linux64&lang=en-US" \
  | tar --extract --verbose --preserve-permissions --bzip2
mkdir -p ~/.local/opt
mv firefox ~/.local/opt
echo $PATH /usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/home/egdoc/.local/bin
PATH=${PATH}:"${HOME}/.local/bin"
ln -s ~/.local/opt/firefox/firefox ~/.local/bin/firefox-dev


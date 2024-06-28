cd /tmp
curl -sLo fastfetch-linux-amd64.deb "https://github.com/fastfetch-cli/fastfetch/releases/latest/download/fastfetch-linux-amd64.deb"
sudo apt install ./fastfetch-linux-amd64.deb
rm fastfetch-linux-amd64.deb
cd -

# Add the official Firefox repo
sudo install -d -m 0755 /etc/apt/keyrings
sudo wget -qO /etc/apt/keyrings/packages.mozilla.org.asc https://packages.mozilla.org/apt/repo-signing-key.gpg
sudo chmod a+r /etc/apt/keyrings/packages.mozilla.org.asc
echo "deb [signed-by=/etc/apt/keyrings/packages.mozilla.org.asc] https://packages.mozilla.org/apt mozilla main" | sudo tee -a /etc/apt/sources.list.d/mozilla.list > /dev/null
echo '
Package: *
Pin: origin packages.mozilla.org
Pin-Priority: 1000
' | sudo tee /etc/apt/preferences.d/mozilla
sudo apt update
sudo apt-get install -y firefox

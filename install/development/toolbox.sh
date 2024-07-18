sudo apt install -y libfuse2

cd /tmp
curl -sLo jetbrains-toolbox.tar.gz "https://download.jetbrains.com/toolbox/jetbrains-toolbox-2.3.2.31487.tar.gz"
tar -xf jetbrains-toolbox.tar.gz
[ ! -d $HOME/Applications ] && mkdir $HOME/Applications
mv ./jetbrains-toolbox-2.3.2.31487/jetbrains-toolbox $HOME/Applications
sudo chmod a+x $HOME/Applications/jetbrains-toolbox
rm -rf jetbrains-toolbox.tar.gz jetbrains-toolbox-2.3.2.31487
cd -






sudo apt install -y zsh zplug

[ -f "~/.zshrc" ] && mv ~/.zshrc ~/.zshrc.bak
cp ~/.local/share/devex/configs/zshrc ~/.zshrc
# chsh -s /bin/zsh
source ~/.local/share/devex/defaults/zsh/shell

[ -f "~/.inputrc" ] && mv ~/.inputrc ~/.inputrc.bak
cp ~/.local/share/devex/configs/inputrc ~/.inputrc

# Install Flatpak package manager
sudo apt install -y flatpak

# Install GNOME Software plugin for Flatpak
sudo apt install -y gnome-software-plugin-flatpak

# Add the Flathub repository for Flatpak if it does not already exist
sudo flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo

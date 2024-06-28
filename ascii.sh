ascii_art='

DDDDDDDDDDDDD                                                 EEEEEEEEEEEEEEEEEEEEEE
D::::::::::::DDD                                              E::::::::::::::::::::E
D:::::::::::::::DD                                            E::::::::::::::::::::E
DDD:::::DDDDD:::::D                                           EE::::::EEEEEEEEE::::E
  D:::::D    D:::::D     eeeeeeeeeeee  vvvvvvv           vvvvvvvE:::::E       EEEEEExxxxxxx      xxxxxxx
  D:::::D     D:::::D  ee::::::::::::ee v:::::v         v:::::v E:::::E              x:::::x    x:::::x
  D:::::D     D:::::D e::::::eeeee:::::eev:::::v       v:::::v  E::::::EEEEEEEEEE     x:::::x  x:::::x
  D:::::D     D:::::De::::::e     e:::::e v:::::v     v:::::v   E:::::::::::::::E      x:::::xx:::::x
  D:::::D     D:::::De:::::::eeeee::::::e  v:::::v   v:::::v    E:::::::::::::::E       x::::::::::x
  D:::::D     D:::::De:::::::::::::::::e    v:::::v v:::::v     E::::::EEEEEEEEEE        x::::::::x
  D:::::D     D:::::De::::::eeeeeeeeeee      v:::::v:::::v      E:::::E                  x::::::::x
  D:::::D    D:::::D e:::::::e                v:::::::::v       E:::::E       EEEEEE    x::::::::::x
DDD:::::DDDDD:::::D  e::::::::e                v:::::::v      EE::::::EEEEEEEE:::::E   x:::::xx:::::x
D:::::::::::::::DD    e::::::::eeeeeeee         v:::::v       E::::::::::::::::::::E  x:::::x  x:::::x
D::::::::::::DDD       ee:::::::::::::e          v:::v        E::::::::::::::::::::E x:::::x    x:::::x
DDDDDDDDDDDDD            eeeeeeeeeeeeee           vvv         EEEEEEEEEEEEEEEEEEEEEExxxxxxx      xxxxxxx

'

# Define the color gradient (shades of cyan and blue)
colors=(
	'\033[38;5;81m' # Cyan
	'\033[38;5;75m' # Light Blue
	'\033[38;5;69m' # Sky Blue
	'\033[38;5;63m' # Dodger Blue
	'\033[38;5;57m' # Deep Sky Blue
	'\033[38;5;51m' # Cornflower Blue
	'\033[38;5;45m' # Royal Blue
)

# Split the ASCII art into lines
IFS=$'\n' read -rd '' -a lines <<<"$ascii_art"

# Print each line with the corresponding color
for i in "${!lines[@]}"; do
	color_index=$((i % ${#colors[@]}))
	echo -e "${colors[color_index]}${lines[i]}"
done

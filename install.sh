#!/usr/bin/bash
#
#           CasaOS Installer Script (fork build) v0.4.16-fork.1
#   Fork:    https://github.com/anonimo18032000
#   Based on: https://github.com/IceWhaleTech/CasaOS installer
#
#   This installs a MODIFIED build of CasaOS (SFTP mounts, scheduled backups,
#   storage health checks, configurable dashboard refresh rate, HTTPS support,
#   opt-in auto-update of apps) from anonimo18032000's forks instead of the
#   official IceWhaleTech releases. Only CasaOS, CasaOS-Gateway,
#   CasaOS-AppManagement and CasaOS-UI are replaced; every other component
#   (MessageBus, UserService, LocalStorage, CLI, AppStore) still comes from
#   the official upstream releases.
#
#   Usage:
#       $ curl -fsSL <raw-url-of-this-script> | bash
#       or, for a local copy:
#       $ bash get-casaos-fork.sh
#
#   In automated environments, you may want to run as root.
#   This only works on Linux systems.
#
#   NOTE: fork binaries were only built for amd64. On other architectures
#   this script automatically falls back to the official upstream release
#   for CasaOS / CasaOS-Gateway / CasaOS-AppManagement (CasaOS-UI is
#   arch-independent and always comes from the fork).
#
clear
echo -e "\e[0m\c"

# shellcheck disable=SC2016
echo '
   _____                 ____   _____
  / ____|               / __ \ / ____|
 | |     __ _ ___  __ _| |  | | (___
 | |    / _` / __|/ _` | |  | |\___ \
 | |___| (_| \__ \ (_| | |__| |____) |
  \_____\__,_|___/\__,_|\____/|_____/

   --- Modified fork build (anonimo18032000) ---
'
export PATH=/usr/sbin:$PATH
export DEBIAN_FRONTEND=noninteractive

set -e

###############################################################################
# GOLBALS                                                                     #
###############################################################################

((EUID)) && sudo_cmd="sudo"

# shellcheck source=/dev/null
source /etc/os-release

# SYSTEM REQUIREMENTS
readonly MINIMUM_DISK_SIZE_GB="5"
readonly MINIMUM_MEMORY="400"
readonly MINIMUM_DOCKER_VERSION="20"
readonly CASA_DEPANDS_PACKAGE=('wget' 'curl' 'smartmontools' 'parted' 'ntfs-3g' 'net-tools' 'udevil' 'samba' 'cifs-utils' 'mergerfs' 'unzip')
readonly CASA_DEPANDS_COMMAND=('wget' 'curl' 'smartctl' 'parted' 'ntfs-3g' 'netstat' 'udevil' 'smbd' 'mount.cifs' 'mount.mergerfs' 'unzip')

# SYSTEM INFO
PHYSICAL_MEMORY=$(LC_ALL=C free -m | awk '/Mem:/ { print $2 }')
readonly PHYSICAL_MEMORY

FREE_DISK_BYTES=$(LC_ALL=C df -P / | tail -n 1 | awk '{print $4}')
readonly FREE_DISK_BYTES

readonly FREE_DISK_GB=$((FREE_DISK_BYTES / 1024 / 1024))

LSB_DIST=$( ([ -n "${ID_LIKE}" ] && echo "${ID_LIKE}") || ([ -n "${ID}" ] && echo "${ID}"))
readonly LSB_DIST

DIST=$(echo "${ID}")
readonly DIST

UNAME_M="$(uname -m)"
readonly UNAME_M

UNAME_U="$(uname -s)"
readonly UNAME_U

readonly CASA_CONF_PATH=/etc/casaos/gateway.ini
# Uninstall script is hosted in this fork (copied from IceWhaleTech/get); upstream raw URL is the fallback.
readonly CASA_UNINSTALL_URL="https://raw.githubusercontent.com/anonimo18032000/CasaOS/main/uninstall.sh"
readonly CASA_UNINSTALL_FALLBACK_URL="https://raw.githubusercontent.com/IceWhaleTech/get/main/uninstall.sh"
readonly CASA_UNINSTALL_PATH=/usr/bin/casaos-uninstall

# REQUIREMENTS CONF PATH
# Udevil
readonly UDEVIL_CONF_PATH=/etc/udevil/udevil.conf
readonly DEVMON_CONF_PATH=/etc/conf.d/devmon

# COLORS
readonly COLOUR_RESET='\e[0m'
readonly aCOLOUR=(
    '\e[38;5;154m' # green  	| Lines, bullets and separators
    '\e[1m'        # Bold white	| Main descriptions
    '\e[90m'       # Grey		| Credits
    '\e[91m'       # Red		| Update notifications Alert
    '\e[33m'       # Yellow		| Emphasis
)

readonly GREEN_LINE=" ${aCOLOUR[0]}─────────────────────────────────────────────────────$COLOUR_RESET"
readonly GREEN_BULLET=" ${aCOLOUR[0]}-$COLOUR_RESET"
readonly GREEN_SEPARATOR="${aCOLOUR[0]}:$COLOUR_RESET"

# CASAOS VARIABLES
TARGET_ARCH=""
TMP_ROOT=/tmp/casaos-installer
REGION="UNKNOWN"
CASA_DOWNLOAD_DOMAIN="https://github.com/"

# FORK VARIABLES
readonly INSTALLER_REVISION="2026-10-29.2"
readonly FORK_OWNER="anonimo18032000"
readonly FORK_CASAOS_TAG="v0.4.15-fork.1"
readonly FORK_GATEWAY_TAG="v0.4.9-alpha4-fork.1"
readonly FORK_APPMANAGEMENT_TAG="v0.4.10-alpha2-fork.1"
readonly FORK_UI_TAG="v0.4.25-fork.1"

trap 'onCtrlC' INT
onCtrlC() {
    echo -e "${COLOUR_RESET}"
    exit 1
}

###############################################################################
# Helpers                                                                     #
###############################################################################

#######################################
# Custom printing function
# Globals:
#   None
# Arguments:
#   $1 0:OK   1:FAILED  2:INFO  3:NOTICE
#   message
# Returns:
#   None
#######################################

Show() {
    # OK
    if (($1 == 0)); then
        echo -e "${aCOLOUR[2]}[$COLOUR_RESET${aCOLOUR[0]}  OK  $COLOUR_RESET${aCOLOUR[2]}]$COLOUR_RESET $2"
    # FAILED
    elif (($1 == 1)); then
        echo -e "${aCOLOUR[2]}[$COLOUR_RESET${aCOLOUR[3]}FAILED$COLOUR_RESET${aCOLOUR[2]}]$COLOUR_RESET $2"
        exit 1
    # INFO
    elif (($1 == 2)); then
        echo -e "${aCOLOUR[2]}[$COLOUR_RESET${aCOLOUR[0]} INFO $COLOUR_RESET${aCOLOUR[2]}]$COLOUR_RESET $2"
    # NOTICE
    elif (($1 == 3)); then
        echo -e "${aCOLOUR[2]}[$COLOUR_RESET${aCOLOUR[4]}NOTICE$COLOUR_RESET${aCOLOUR[2]}]$COLOUR_RESET $2"
    fi
}

Warn() {
    echo -e "${aCOLOUR[3]}$1$COLOUR_RESET"
}

GreyStart() {
    echo -e "${aCOLOUR[2]}\c"
}

ColorReset() {
    echo -e "$COLOUR_RESET\c"
}

# Clear Terminal
Clear_Term() {

    # Without an input terminal, there is no point in doing this.
    [[ -t 0 ]] || return

    # Printing terminal height - 1 newlines seems to be the fastest method that is compatible with all terminal types.
    lines=$(tput lines) i newlines
    local lines

    for ((i = 1; i < ${lines% *}; i++)); do newlines+='\n'; done
    echo -ne "\e[0m$newlines\e[H"

}

# Check file exists
exist_file() {
    if [ -e "$1" ]; then
        return 1
    else
        return 2
    fi
}

###############################################################################
# FUNCTIONS                                                                   #
###############################################################################



# 1 Check Arch
Check_Arch() {
    case $UNAME_M in
    *aarch64*)
        TARGET_ARCH="arm64"
        ;;
    *64*)
        TARGET_ARCH="amd64"
        ;;
    *armv7*)
        TARGET_ARCH="arm-7"
        ;;
    *)
        Show 1 "Aborted, unsupported or unknown architecture: $UNAME_M"
        exit 1
        ;;
    esac
    Show 0 "Your hardware architecture is : $UNAME_M"

    # The fork's Gateway/AppManagement/CasaOS binaries were only built for
    # amd64. On other architectures we fall back to the matching official
    # upstream release for those three, and only override CasaOS-UI (which
    # is a "linux-all" arch-independent static build).
    if [[ "$TARGET_ARCH" == "amd64" ]]; then
        GATEWAY_PKG="https://github.com/${FORK_OWNER}/CasaOS-Gateway/releases/download/${FORK_GATEWAY_TAG}/linux-amd64-casaos-gateway-${FORK_GATEWAY_TAG}.tar.gz"
        APPMANAGEMENT_PKG="https://github.com/${FORK_OWNER}/CasaOS-AppManagement/releases/download/${FORK_APPMANAGEMENT_TAG}/linux-amd64-casaos-app-management-${FORK_APPMANAGEMENT_TAG}.tar.gz"
        CASAOS_PKG="https://github.com/${FORK_OWNER}/CasaOS/releases/download/${FORK_CASAOS_TAG}/linux-amd64-casaos-${FORK_CASAOS_TAG}.tar.gz"
    else
        Show 3 "Fork binaries are amd64-only; using official upstream releases for CasaOS/Gateway/AppManagement on $TARGET_ARCH."
        GATEWAY_PKG="${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-Gateway/releases/download/v0.4.9-alpha4/linux-${TARGET_ARCH}-casaos-gateway-v0.4.9-alpha4.tar.gz"
        APPMANAGEMENT_PKG="${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-AppManagement/releases/download/v0.4.10-alpha2/linux-${TARGET_ARCH}-casaos-app-management-v0.4.10-alpha2.tar.gz"
        CASAOS_PKG="${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS/releases/download/v0.4.15/linux-${TARGET_ARCH}-casaos-v0.4.15.tar.gz"
    fi

    # The fork UI talks to fork-only endpoints (SFTP, backups, HTTPS), so it is only used together
    # with the fork's amd64 backend; other architectures get the matching upstream UI.
    if [[ "$TARGET_ARCH" == "amd64" ]]; then
        UI_PKG="https://github.com/${FORK_OWNER}/CasaOS-UI/releases/download/${FORK_UI_TAG}/linux-all-casaos-${FORK_UI_TAG}.tar.gz"
    else
        UI_PKG="${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-UI/releases/download/v0.4.25/linux-all-casaos-v0.4.25.tar.gz"
    fi

    CASA_PACKAGES=(
        "${GATEWAY_PKG}"
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-MessageBus/releases/download/v0.4.4-3-alpha2/linux-${TARGET_ARCH}-casaos-message-bus-v0.4.4-3-alpha2.tar.gz"
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-UserService/releases/download/v0.4.8/linux-${TARGET_ARCH}-casaos-user-service-v0.4.8.tar.gz"
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-LocalStorage/releases/download/v0.4.4/linux-${TARGET_ARCH}-casaos-local-storage-v0.4.4.tar.gz"
        "${APPMANAGEMENT_PKG}"
        "${CASAOS_PKG}"
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-CLI/releases/download/v0.4.4-3-alpha1/linux-${TARGET_ARCH}-casaos-cli-v0.4.4-3-alpha1.tar.gz"
        "${UI_PKG}"
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-AppStore/releases/download/v0.4.5/linux-all-appstore-v0.4.5.tar.gz"
    )
}

# SHA-256 of every package the installer downloads. A package that is missing here or does not
# match aborts the install. When a fork release is rebuilt, update its entry (GitHub shows the
# digest on the release asset; for others run sha256sum on the downloaded file).
declare -A CASA_SHA256=(
    ["linux-all-appstore-v0.4.5.tar.gz"]="4f346a057097c09706813052c3ef5d0ef49b609aea320b4fcf3af7609b3957b5"
    ["linux-all-casaos-v0.4.25-fork.1.tar.gz"]="511caf8504dc9578fb67893e0063c91849b5a1f4e46851aa35b628f6aa317fe4"
    ["linux-all-casaos-v0.4.25.tar.gz"]="390d399ba89aeba9ce2747ea58348acb0c86740fb55a6e617eb42a1d3f715bf5"
    ["linux-amd64-casaos-app-management-v0.4.10-alpha2-fork.1.tar.gz"]="c2c58027aea87ce834cb3d8b79ae053aa9d344a804226e3940c622fcd304ccdd"
    ["linux-amd64-casaos-app-management-v0.4.10-alpha2.tar.gz"]="161557e5fbe9066183746798af090f62d33da7b80f9b1ad5159e9fe9dbdfbe2b"
    ["linux-amd64-casaos-cli-v0.4.4-3-alpha1.tar.gz"]="d25ad6bbd68f555f20ec0ab4a0984ca2691a68e112562dc613d5707493173a0d"
    ["linux-amd64-casaos-gateway-v0.4.9-alpha4-fork.1.tar.gz"]="25fb8877623e202000a03e8e9ff9b1041d8c30fb672ebadbe2173625376eb138"
    ["linux-amd64-casaos-gateway-v0.4.9-alpha4.tar.gz"]="458e66bdd8f8ad63fe5610c01dd76bb76d5054c899c92f2bba31be36d769c798"
    ["linux-amd64-casaos-local-storage-v0.4.4.tar.gz"]="523edb627ec11b1bc74d408a6929028903ff404115d316d1fb7f40cf819ba831"
    ["linux-amd64-casaos-message-bus-v0.4.4-3-alpha2.tar.gz"]="29823621aaa33dd22271f436e741f5959a7df6a7d5540036a5da866de0f7c7fe"
    ["linux-amd64-casaos-user-service-v0.4.8.tar.gz"]="62f3d12d93215af58af3d9da4448b244b33b533a0321afa46229fde1f6d66bab"
    ["linux-amd64-casaos-v0.4.15-fork.1.tar.gz"]="4d28ae5afa52e54f7545c0299577a0432d5968b0d3e4790ab605bfd6c44ede8c"
    ["linux-amd64-casaos-v0.4.15.tar.gz"]="8e5bd4372fa8a32e6c7c4f35617d27613fcce7a6be25ba92593c81cade3b2317"
    ["linux-arm-7-casaos-app-management-v0.4.10-alpha2.tar.gz"]="9229b04f0bb62c76806578efcd84fea3df05c5473e51c6b1972aa92f216b9fad"
    ["linux-arm-7-casaos-cli-v0.4.4-3-alpha1.tar.gz"]="3a92be9f37471c89e28cf30f32864aa93816a54dc28440645c48369f598a1b3e"
    ["linux-arm-7-casaos-gateway-v0.4.9-alpha4.tar.gz"]="a9028a0bdb8e0bc38f0b85910cd588d983a0af88d4b43dd047d6059552ce1e9b"
    ["linux-arm-7-casaos-local-storage-v0.4.4.tar.gz"]="4499289a19c078c9016bf6a6defb167b559aae6594b8035feaa5345c372b9b6d"
    ["linux-arm-7-casaos-message-bus-v0.4.4-3-alpha2.tar.gz"]="b608c4dd601a0c41608eb1a443e83b5b85b9b4b89bacfb20e6174333fac2a065"
    ["linux-arm-7-casaos-user-service-v0.4.8.tar.gz"]="6228e6a6cdf625ce775a7405516822e50bfe6773fc6ee267fc21440686fdd1bb"
    ["linux-arm-7-casaos-v0.4.15.tar.gz"]="e294232d173ba6102c658ad3af2f7eec53c6a24188b2f03a046a57263f593fb7"
    ["linux-arm64-casaos-app-management-v0.4.10-alpha2.tar.gz"]="014437d988f072b95cc0dd14f9df4a7b49bdc980a2a4a6bb05d8acea6abaa984"
    ["linux-arm64-casaos-cli-v0.4.4-3-alpha1.tar.gz"]="30f05faa9607c59942d149d31fac15b7c13a19f2b785d937605f0fdc246a47fb"
    ["linux-arm64-casaos-gateway-v0.4.9-alpha4.tar.gz"]="9fb893eab310996d88ffab7081f3e026d92aa4d27e51b8c2ee5ce19171b42ab9"
    ["linux-arm64-casaos-local-storage-v0.4.4.tar.gz"]="b023987bf24251ede5c7d57bdbbe4b8f6ed12cfdbe1ad9e7704bb8f15ac382d2"
    ["linux-arm64-casaos-message-bus-v0.4.4-3-alpha2.tar.gz"]="9ea763674a21340c874397aa81fa5fde7e4d5ada574b14fe88871956fa066ecf"
    ["linux-arm64-casaos-user-service-v0.4.8.tar.gz"]="5396e92f74398801915f446c179cc18b949e0be0a3667bbb104ac9f2c650dfca"
    ["linux-arm64-casaos-v0.4.15.tar.gz"]="9d5fa2c46278f83b08b2611599a0f1fe15cfcf27ca89aa8d2daa8f88d3dd0a26"
)

# PACKAGE LIST OF CASAOS (make sure the services are in the right order)
CASA_SERVICES=(
    "casaos-gateway.service"
"casaos-message-bus.service"
"casaos-user-service.service"
"casaos-local-storage.service"
"casaos-app-management.service"
"rclone.service"
"casaos.service"  # must be the last one so update from UI can work
)

# 2 Check Distribution
Check_Distribution() {
    sType=0
    notice=""
    case $LSB_DIST in
    *debian*) ;;

    *ubuntu*) ;;

    *raspbian*) ;;

    *openwrt*)
        Show 1 "Aborted, OpenWrt cannot be installed using this script."
        exit 1
        ;;
    *alpine*)
        Show 1 "Aborted, Alpine installation is not yet supported."
        exit 1
        ;;
    *trisquel*) ;;

    *)
        sType=3
        notice="We have not tested it on this system and it may fail to install."
        ;;
    esac
    Show ${sType} "Your Linux Distribution is : ${DIST} ${notice}"

    if [[ ${sType} == 1 ]]; then
        select yn in "Yes" "No"; do
            case $yn in
            [yY][eE][sS] | [yY])
                Show 0 "Distribution check has been ignored."
                break
                ;;
            [nN][oO] | [nN])
                Show 1 "Already exited the installation."
                exit 1
                ;;
            esac
        done < /dev/tty # < /dev/tty is used to read the input from the terminal
    fi
}

# 3 Check OS
Check_OS() {
    if [[ $UNAME_U == *Linux* ]]; then
        Show 0 "Your System is : $UNAME_U"
    else
        Show 1 "This script is only for Linux."
        exit 1
    fi
}

# 4 Check Memory
Check_Memory() {
    if [[ "${PHYSICAL_MEMORY}" -lt "${MINIMUM_MEMORY}" ]]; then
        Show 1 "requires atleast 400MB physical memory."
        exit 1
    fi
    Show 0 "Memory capacity check passed."
}

# 5 Check Disk
Check_Disk() {
    if [[ "${FREE_DISK_GB}" -lt "${MINIMUM_DISK_SIZE_GB}" ]]; then
        echo -e "${aCOLOUR[4]}Recommended free disk space is greater than ${MINIMUM_DISK_SIZE_GB}GB, Current free disk space is ${aCOLOUR[3]}${FREE_DISK_GB}GB${COLOUR_RESET}${aCOLOUR[4]}.\nContinue installation?${COLOUR_RESET}"
        select yn in "Yes" "No"; do
            case $yn in
            [yY][eE][sS] | [yY])
                Show 0 "Disk capacity check has been ignored."
                break
                ;;
            [nN][oO] | [nN])
                Show 1 "Already exited the installation."
                exit 1
                ;;
            esac
        done < /dev/tty  # < /dev/tty is used to read the input from the terminal
    else
        Show 0 "Disk capacity check passed."
    fi
}

# Check Port Use
Check_Port() {
    TCPListeningnum=$(${sudo_cmd} netstat -an | grep ":$1 " | awk '$1 == "tcp" && $NF == "LISTEN" {print $0}' | wc -l)
    UDPListeningnum=$(${sudo_cmd} netstat -an | grep ":$1 " | awk '$1 == "udp" && $NF == "0.0.0.0:*" {print $0}' | wc -l)
    ((Listeningnum = TCPListeningnum + UDPListeningnum))
    if [[ $Listeningnum == 0 ]]; then
        echo "0"
    else
        echo "1"
    fi
}

# Get an available port
Get_Port() {
    CurrentPort=$(${sudo_cmd} cat ${CASA_CONF_PATH} | grep HttpPort | awk '{print $3}')
    if [[ $CurrentPort == "$Port" ]]; then
        for PORT in {80..65536}; do
            if [[ $(Check_Port "$PORT") == 0 ]]; then
                Port=$PORT
                break
            fi
        done
    else
        Port=$CurrentPort
    fi
}

# Update package

Update_Package_Resource() {
    Show 2 "Updating package manager..."
    GreyStart
    if [ -x "$(command -v apk)" ]; then
        ${sudo_cmd} apk update
    elif [ -x "$(command -v apt-get)" ]; then
        ${sudo_cmd} apt-get update -qq
    elif [ -x "$(command -v dnf)" ]; then
        ${sudo_cmd} dnf check-update
    elif [ -x "$(command -v zypper)" ]; then
        ${sudo_cmd} zypper update
    elif [ -x "$(command -v yum)" ]; then
        ${sudo_cmd} yum update
    fi
    ColorReset
    Show 0 "Update package manager complete."
}

# Install depends package
Install_Depends() {
    for ((i = 0; i < ${#CASA_DEPANDS_COMMAND[@]}; i++)); do
        cmd=${CASA_DEPANDS_COMMAND[i]}
        if [[ ! -x $(${sudo_cmd} which "$cmd") ]]; then
            packagesNeeded=${CASA_DEPANDS_PACKAGE[i]}
            Show 2 "Install the necessary dependencies: \e[33m$packagesNeeded \e[0m"
            GreyStart
            if [ -x "$(command -v apk)" ]; then
                ${sudo_cmd} apk add --no-cache "$packagesNeeded"
            elif [ -x "$(command -v apt-get)" ]; then
                ${sudo_cmd} apt-get -y -qq install "$packagesNeeded" --no-upgrade
            elif [ -x "$(command -v dnf)" ]; then
                ${sudo_cmd} dnf install "$packagesNeeded"
            elif [ -x "$(command -v zypper)" ]; then
                ${sudo_cmd} zypper install "$packagesNeeded"
            elif [ -x "$(command -v yum)" ]; then
                ${sudo_cmd} yum install -y "$packagesNeeded"
            elif [ -x "$(command -v pacman)" ]; then
                ${sudo_cmd} pacman -S "$packagesNeeded"
            elif [ -x "$(command -v paru)" ]; then
                ${sudo_cmd} paru -S "$packagesNeeded"
            else
                Show 1 "Package manager not found. You must manually install: \e[33m$packagesNeeded \e[0m"
            fi
            ColorReset
        fi
    done
}

Check_Dependency_Installation() {
    for ((i = 0; i < ${#CASA_DEPANDS_COMMAND[@]}; i++)); do
        cmd=${CASA_DEPANDS_COMMAND[i]}
        if [[ ! -x $(${sudo_cmd} which "$cmd") ]]; then
            packagesNeeded=${CASA_DEPANDS_PACKAGE[i]}
            Show 1 "Dependency \e[33m$packagesNeeded \e[0m installation failed, please try again manually!"
            exit 1
        fi
    done
}

# Check Docker running
Check_Docker_Running() {
    for ((i = 1; i <= 3; i++)); do
        sleep 3
        if [[ ! $(${sudo_cmd} systemctl is-active docker) == "active" ]]; then
            Show 3 "Docker is not running, trying to start it..."
            ${sudo_cmd} systemctl start docker || true
        else
            break
        fi
    done
}

#Check Docker Installed and version
Check_Docker_Install() {
    if [[ -x "$(command -v docker)" ]]; then
        Docker_Version=$(${sudo_cmd} docker version --format '{{.Server.Version}}')
        if [[ $? -ne 0 ]]; then
            Install_Docker
        elif [[ ${Docker_Version:0:2} -lt "${MINIMUM_DOCKER_VERSION}" ]]; then
            Show 1 "Recommended minimum Docker version is \e[33m${MINIMUM_DOCKER_VERSION}.xx.xx\e[0m,\nCurrent Docker version is \e[33m${Docker_Version}\e[0m,\nPlease uninstall current Docker and rerun the CasaOS installation script."
            exit 1
        else
            Show 0 "Current Docker version is ${Docker_Version}."
        fi
    else
        Install_Docker
    fi
}

# Check Docker installed
Check_Docker_Install_Final() {
    if [[ -x "$(command -v docker)" ]]; then
        Docker_Version=$(${sudo_cmd} docker version --format '{{.Server.Version}}')
        if [[ $? -ne 0 ]]; then
            Install_Docker
        elif [[ ${Docker_Version:0:2} -lt "${MINIMUM_DOCKER_VERSION}" ]]; then
            Show 1 "Recommended minimum Docker version is \e[33m${MINIMUM_DOCKER_VERSION}.xx.xx\e[0m,\nCurrent Docker version is \e[33m${Docker_Version}\e[0m,\nPlease uninstall current Docker and rerun the CasaOS installation script."
            exit 1
        else
            Show 0 "Current Docker version is ${Docker_Version}."
            Check_Docker_Running
        fi
    else
        Show 1 "Installation failed, please run 'curl -fsSL https://get.docker.com | bash' and rerun the CasaOS installation script."
        exit 1
    fi
}

#Install Docker
Install_Docker() {
    Show 2 "Install the necessary dependencies: \e[33mDocker \e[0m"
    if [[ ! -d "${PREFIX}/etc/apt/sources.list.d" ]]; then
        ${sudo_cmd} mkdir -p "${PREFIX}/etc/apt/sources.list.d"
    fi
    GreyStart
    ${sudo_cmd} curl -fsSL https://get.docker.com | bash
    ColorReset
    if [[ $? -ne 0 ]]; then
        Show 1 "Installation failed, please try again."
        exit 1
    else
        Check_Docker_Install_Final
    fi
}

###############################################################################
# Docker API override for CasaOS                                             #
###############################################################################

Apply_Docker_API_Override() {
    Show 2 "Applying Docker API compatibility override for CasaOS..."
    local override_dir="/etc/systemd/system/docker.service.d"
    local override_file="${override_dir}/override.conf"
    local wanted=$'[Service]
Environment=DOCKER_MIN_API_VERSION=1.24
'

    # Restarting Docker restarts every container, so only touch it when the override is missing or
    # different (e.g. not on re-runs / updates started from the CasaOS UI).
    if [[ -f "${override_file}" && "$(cat "${override_file}")" == "${wanted%$'
'}" ]]; then
        Show 0 "Docker API override already in place."
        return
    fi

    ${sudo_cmd} mkdir -p "${override_dir}" || Show 3 "Failed to create ${override_dir} (non-fatal)."
    printf '%s' "${wanted}" | ${sudo_cmd} tee "${override_file}" >/dev/null
    Show 0 "Docker API override written to ${override_file}"

    ${sudo_cmd} systemctl daemon-reload || Show 3 "systemd daemon-reload failed (non-fatal)."
    if ! ${sudo_cmd} systemctl restart docker; then
        Show 3 "Failed to restart Docker after API override. Please check 'systemctl status docker'."
    else
        Show 0 "Docker restarted with API override. CasaOS should now be able to talk to newer Docker versions."
    fi
}

###############################################################################
# Rclone & other components                                                  #
###############################################################################

#Install Rclone
Install_rclone_from_source() {
  local rclone_version="v1.61.1"
  local zip_name="rclone-${rclone_version}-linux-amd64.zip"
  local rclone_tmp
  rclone_tmp=$(mktemp -d)
  if ! ${sudo_cmd} wget -q -O "${rclone_tmp}/${zip_name}" "https://downloads.rclone.org/${rclone_version}/${zip_name}" &&
    ! ${sudo_cmd} wget -q -O "${rclone_tmp}/${zip_name}" "https://github.com/rclone/rclone/releases/download/${rclone_version}/${zip_name}"; then
    rm -rf "${rclone_tmp}"
    Show 1 "Failed to download rclone ${rclone_version}, please try again."
    exit 1
  fi
  unzip -q -o "${rclone_tmp}/${zip_name}" -d "${rclone_tmp}" || {
    rm -rf "${rclone_tmp}"
    Show 1 "Failed to extract rclone ${rclone_version}."
    exit 1
  }
  local rclone_dir="${rclone_tmp}/rclone-${rclone_version}-linux-amd64"
  ${sudo_cmd} install -m 0755 "${rclone_dir}/rclone" "${PREFIX}/usr/bin/rclone" || {
    rm -rf "${rclone_tmp}"
    Show 1 "Failed to install rclone binary."
    exit 1
  }
  if [[ -f "${rclone_dir}/rclone.1" ]]; then
    ${sudo_cmd} mkdir -p "${PREFIX}/usr/share/man/man1"
    ${sudo_cmd} install -m 0644 "${rclone_dir}/rclone.1" "${PREFIX}/usr/share/man/man1/rclone.1"
  fi
  rm -rf "${rclone_tmp}"
  Show 0 "Rclone ${rclone_version} installed successfully."
}

Install_Rclone() {
  Show 2 "Install the necessary dependencies: Rclone"
  if [[ -x "$(command -v rclone)" ]]; then
    version=$(rclone --version 2>>errors | head -n 1)
    target_version="rclone v1.61.1"
    rclone1="${PREFIX}/usr/share/man/man1/rclone.1.gz"
    if [ "$version" != "$target_version" ]; then
      Show 3 "Will change rclone from $version to $target_version."
      rclone_path=$(command -v rclone)
      ${sudo_cmd} rm -rf "${rclone_path}"
      if [[ -f "$rclone1" ]]; then
        ${sudo_cmd} rm -rf "$rclone1"
      fi
      Install_rclone_from_source
    else
      Show 2 "Target version already installed."
    fi
  else
    Install_rclone_from_source
  fi
  ${sudo_cmd} systemctl enable rclone || Show 3 "Service rclone does not exist."
}

#Configuration Addons
Configuration_Addons() {
    Show 2 "Configuration CasaOS Addons"
    #Remove old udev rules
    if [[ -f "${PREFIX}/etc/udev/rules.d/11-usb-mount.rules" ]]; then
        ${sudo_cmd} rm -rf "${PREFIX}/etc/udev/rules.d/11-usb-mount.rules"
    fi

    if [[ -f "${PREFIX}/etc/systemd/system/usb-mount@.service" ]]; then
        ${sudo_cmd} rm -rf "${PREFIX}/etc/systemd/system/usb-mount@.service"
    fi

    #Udevil
    if [[ -f $PREFIX${UDEVIL_CONF_PATH} ]]; then

        # GreyStart
        # Add a devmon user
        USERNAME=devmon
        id ${USERNAME} &>/dev/null || {
            ${sudo_cmd} useradd -M -u 300 ${USERNAME}
            ${sudo_cmd} usermod -L ${USERNAME}
        }

        ${sudo_cmd} sed -i '/exfat/s/, nonempty//g' "$PREFIX"${UDEVIL_CONF_PATH}
        ${sudo_cmd} sed -i '/default_options/s/, noexec//g' "$PREFIX"${UDEVIL_CONF_PATH}
        ${sudo_cmd} sed -i '/^ARGS/cARGS="--mount-options nosuid,nodev,noatime --ignore-label EFI"' "$PREFIX"${DEVMON_CONF_PATH}

        # Add and start Devmon service
        GreyStart
        ${sudo_cmd} systemctl enable devmon@devmon
        ${sudo_cmd} systemctl start devmon@devmon
        ColorReset
        # ColorReset
    fi
}

# Download And Install CasaOS
DownloadAndInstallCasaOS() {
    if [ -z "${BUILD_DIR}" ]; then
        ${sudo_cmd} rm -rf ${TMP_ROOT}
        mkdir -p ${TMP_ROOT} || Show 1 "Failed to create temporary directory"
        TMP_DIR=$(${sudo_cmd} mktemp -d -p ${TMP_ROOT} || Show 1 "Failed to create temporary directory")

        pushd "${TMP_DIR}"

        for PACKAGE in "${CASA_PACKAGES[@]}"; do
            Show 2 "Downloading ${PACKAGE}..."
            GreyStart
            ${sudo_cmd} wget -t 3 -q --show-progress -c  "${PACKAGE}" || Show 1 "Failed to download package"
            ColorReset

            PACKAGE_NAME=$(basename "${PACKAGE}")
            EXPECTED_SHA256="${CASA_SHA256[${PACKAGE_NAME}]:-}"
            if [[ -z "${EXPECTED_SHA256}" ]]; then
                Show 1 "No known checksum for ${PACKAGE_NAME}; refusing to install an unverified package."
            fi
            ACTUAL_SHA256=$(sha256sum "${PACKAGE_NAME}" | cut -d' ' -f1)
            if [[ "${ACTUAL_SHA256}" != "${EXPECTED_SHA256}" ]]; then
                Show 1 "Checksum mismatch for ${PACKAGE_NAME} (expected ${EXPECTED_SHA256}, got ${ACTUAL_SHA256})."
            fi
        done

        for PACKAGE_FILE in linux-*.tar.gz; do
            Show 2 "Extracting ${PACKAGE_FILE}..."
            GreyStart
            ${sudo_cmd} tar zxf "${PACKAGE_FILE}" || Show 1 "Failed to extract package"
            ColorReset
        done

        # Some release archives were packaged on Windows and may contain
        # CRLF line endings, which break `bash` when running these scripts,
        # and break curl when a URL is read from a CRLF-terminated *.list file.
        ${sudo_cmd} find "${TMP_DIR}" \( -name "*.sh" -o -name "*.list" \) -exec sed -i 's/\r$//' {} \;

        BUILD_DIR=$(${sudo_cmd} realpath -e "${TMP_DIR}"/build || Show 1 "Failed to find build directory")

        popd
    fi

    for SERVICE in "${CASA_SERVICES[@]}"; do
        if ${sudo_cmd} systemctl --quiet is-active "${SERVICE}"; then
            Show 2 "Stopping ${SERVICE}..."
            GreyStart
            ${sudo_cmd} systemctl stop "${SERVICE}" || Show 3 "Service ${SERVICE} does not exist."
            ColorReset
        fi
    done

    MIGRATION_SCRIPT_DIR=$(realpath -e "${BUILD_DIR}"/scripts/migration/script.d || Show 1 "Failed to find migration script directory")

    for MIGRATION_SCRIPT in "${MIGRATION_SCRIPT_DIR}"/*.sh; do
        Show 2 "Running ${MIGRATION_SCRIPT}..."

        ${sudo_cmd} bash "${MIGRATION_SCRIPT}" || Show 1 "Failed to run migration script"

    done


    Show 2 "Installing CasaOS..."
    SYSROOT_DIR=$(realpath -e "${BUILD_DIR}"/sysroot || Show 1 "Failed to find sysroot directory")

    # Generate manifest for uninstallation
    MANIFEST_FILE=${BUILD_DIR}/sysroot/var/lib/casaos/manifest
    ${sudo_cmd} touch "${MANIFEST_FILE}" || Show 1 "Failed to create manifest file"

    GreyStart
    find "${SYSROOT_DIR}" -type f | ${sudo_cmd} cut -c ${#SYSROOT_DIR}- | ${sudo_cmd} cut -c 2- | ${sudo_cmd} tee "${MANIFEST_FILE}" >/dev/null || Show 1 "Failed to create manifest file"

    ${sudo_cmd} cp -rf "${SYSROOT_DIR}"/* / || Show 1 "Failed to install CasaOS"
    ColorReset

    SETUP_SCRIPT_DIR=$(realpath -e "${BUILD_DIR}"/scripts/setup/script.d || Show 1 "Failed to find setup script directory")

    for SETUP_SCRIPT in "${SETUP_SCRIPT_DIR}"/*.sh; do
        Show 2 "Running ${SETUP_SCRIPT}..."
        GreyStart
        ${sudo_cmd} bash "${SETUP_SCRIPT}" || Show 1 "Failed to run setup script"
        ColorReset
    done

    UI_EVENTS_REG_SCRIPT=/etc/casaos/start.d/register-ui-events.sh
    if [[ -f ${UI_EVENTS_REG_SCRIPT} ]]; then
        ${sudo_cmd} chmod +x $UI_EVENTS_REG_SCRIPT
    fi

    #Download Uninstall Script (optional: a failure here must not abort the install)
    ${sudo_cmd} rm -f "$PREFIX/tmp/casaos-uninstall"
    if ${sudo_cmd} curl -fsSL --retry 3 "$CASA_UNINSTALL_URL" -o "$PREFIX/tmp/casaos-uninstall" ||
        ${sudo_cmd} curl -fsSL --retry 3 "$CASA_UNINSTALL_FALLBACK_URL" -o "$PREFIX/tmp/casaos-uninstall"; then
        ${sudo_cmd} cp -f "$PREFIX/tmp/casaos-uninstall" "$CASA_UNINSTALL_PATH" &&
            ${sudo_cmd} chmod +x "$CASA_UNINSTALL_PATH" ||
            Show 3 "Could not install the uninstall script to ${CASA_UNINSTALL_PATH}."
    else
        Show 3 "Could not download the uninstall script; continuing without it."
    fi

    Install_Rclone

    for SERVICE in "${CASA_SERVICES[@]}"; do
        Show 2 "Starting ${SERVICE}..."
        GreyStart
        ${sudo_cmd} systemctl start "${SERVICE}" || Show 3 "Service ${SERVICE} does not exist."
        ColorReset
    done
}

Clean_Temp_Files() {
    Show 2 "Clean temporary files..."
    [[ -n "${TMP_DIR:-}" ]] && { ${sudo_cmd} rm -rf "${TMP_DIR}" || Show 1 "Failed to clean temporary files"; }
    return 0
}

Check_Service_status() {
    for SERVICE in "${CASA_SERVICES[@]}"; do
        Show 2 "Checking ${SERVICE}..."
        if [[ $(${sudo_cmd} systemctl is-active "${SERVICE}") == "active" ]]; then
            Show 0 "${SERVICE} is running."
        else
            Show 1 "${SERVICE} is not running, Please reinstall."
            exit 1
        fi
    done
}

# Get the physical NIC IP
Get_IPs() {
    local conf
    conf=$(${sudo_cmd} cat "${CASA_CONF_PATH}" 2>/dev/null || true)
    # exact keys only: gateway.ini also has tlsport=, which a plain `grep port` would match
    local PORT TLS_ENABLED TLS_PORT
    PORT=$(echo "${conf}" | grep -E '^port[[:space:]]*=' | head -n 1 | cut -d= -f2 | tr -d '[:space:]' || true)
    TLS_ENABLED=$(echo "${conf}" | grep -E '^tlsenabled[[:space:]]*=' | head -n 1 | cut -d= -f2 | tr -d '[:space:]' || true)
    TLS_PORT=$(echo "${conf}" | grep -E '^tlsport[[:space:]]*=' | head -n 1 | cut -d= -f2 | tr -d '[:space:]' || true)
    [[ "${PORT}" =~ ^[0-9]+$ ]] || PORT=80
    [[ "${TLS_PORT}" =~ ^[0-9]+$ ]] || TLS_PORT=443

    local ALL_NIC NIC IP
    ALL_NIC=$(${sudo_cmd} ls /sys/class/net/ | grep -v "$(ls /sys/devices/virtual/net/)" || true)
    for NIC in ${ALL_NIC}; do
        IP=$(${sudo_cmd} ip -4 -o addr show dev "${NIC}" 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -n 1 || true)
        if [[ -n ${IP} ]]; then
            if [[ "${TLS_ENABLED}" == "true" ]]; then
                if [[ "${TLS_PORT}" -eq 443 ]]; then
                    echo -e "${GREEN_BULLET} https://${IP} (${NIC})"
                else
                    echo -e "${GREEN_BULLET} https://${IP}:${TLS_PORT} (${NIC})"
                fi
            elif [[ "${PORT}" -eq 80 ]]; then
                echo -e "${GREEN_BULLET} http://${IP} (${NIC})"
            else
                echo -e "${GREEN_BULLET} http://${IP}:${PORT} (${NIC})"
            fi
        fi
    done
}

# Show Welcome Banner
Welcome_Banner() {
    CASA_TAG=$(casaos -v)

    echo -e "${GREEN_LINE}${aCOLOUR[1]}"
    echo -e " CasaOS ${CASA_TAG}${COLOUR_RESET} (modified fork build) is running at${COLOUR_RESET}${GREEN_SEPARATOR}"
    echo -e "${GREEN_LINE}"
    Get_IPs
    echo -e " Open your browser and visit the above address."
    echo -e "${GREEN_LINE}"
    echo -e ""
    echo -e " ${aCOLOUR[2]}Fork Project    : https://github.com/${FORK_OWNER} (installer ${INSTALLER_REVISION})"
    echo -e " ${aCOLOUR[2]}Upstream Project: https://github.com/IceWhaleTech/CasaOS"
    echo -e ""
    echo -e " ${COLOUR_RESET}${aCOLOUR[1]}Uninstall       ${COLOUR_RESET}: casaos-uninstall"
    echo -e "${COLOUR_RESET}"
}

###############################################################################
# Main                                                                        #
###############################################################################

#Usage
usage() {
    cat <<-EOF
		Usage: install.sh [options]
		Valid options are:
		    -p <build_dir>          Specify build directory (Local install)
		    -h                      Show this help message and exit
	EOF
    exit "$1"
}

while getopts ":p:h" arg; do
    case "$arg" in
    p)
        BUILD_DIR=$OPTARG
        ;;
    h)
        usage 0
        ;;
    *)
        usage 1
        ;;
    esac
done

# Step 1: Check ARCH
Check_Arch

# Step 2: Check OS
Check_OS

# Step 3: Check Distribution
Check_Distribution

# Step 4: Check System Required
Check_Memory
Check_Disk

# Step 5: Install Depends
Update_Package_Resource
Install_Depends
Check_Dependency_Installation

# Step 6: Check And Install Docker
Check_Docker_Install
Apply_Docker_API_Override

# Step 7: Configuration Addon
Configuration_Addons

# Step 8: Download And Install CasaOS
DownloadAndInstallCasaOS

# Step 9: Check Service Status
Check_Service_status

# Step 10: Remove temporary files, then show the welcome banner
Clean_Temp_Files
Welcome_Banner

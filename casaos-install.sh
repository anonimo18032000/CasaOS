#!/usr/bin/bash
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
                                      
   --- Modificado por DanielSantos ---
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

# REQUISITOS DO SISTEMA
readonly MINIMUM_DISK_SIZE_GB="5"
readonly MINIMUM_MEMORY="400"
readonly MINIMUM_DOCKER_VERSION="20"
readonly CASA_DEPANDS_PACKAGE=('wget' 'curl' 'smartmontools' 'parted' 'ntfs-3g' 'net-tools' 'udevil' 'samba' 'cifs-utils' 'mergerfs' 'unzip')
readonly CASA_DEPANDS_COMMAND=('wget' 'curl' 'smartctl' 'parted' 'ntfs-3g' 'netstat' 'udevil' 'smbd' 'mount.cifs' 'mount.mergerfs' 'unzip')

# INFORMAÇÕES DO SISTEMA
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
readonly CASA_UNINSTALL_URL="https://get.casaos.io/uninstall/v0.4.15"
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

trap 'onCtrlC' INT
onCtrlC() {
    echo -e "${COLOUR_RESET}"
    exit 1
}

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



# 0 Get download url domain
# To solve the problem that Chinese users cannot access github.
Get_Download_Url_Domain() {
    # Use ipconfig.io/country and https://ifconfig.io/country_code to get the country code
    REGION=$(${sudo_cmd} curl --connect-timeout 2 -s ipconfig.io/country || echo "")
    if [ "${REGION}" = "" ]; then
       REGION=$(${sudo_cmd} curl --connect-timeout 2 -s https://ifconfig.io/country_code || echo "")
    fi
    if [[ "${REGION}" = "China" ]] || [[ "${REGION}" = "CN" ]]; then
        CASA_DOWNLOAD_DOMAIN="https://casaos.oss-cn-shanghai.aliyuncs.com/"
    fi
}

# 1 Check Arch
Check_Arch() {
    case $UNAME_M in
    *64*)
        TARGET_ARCH="amd64"
        ;;
    *)
        Show 1 "Arquitetura abortada, sem suporte ou desconhecida: $UNAME_M"
        exit 1
        ;;
    esac
    Show 0 "A sua arquitetura de hardware é : $UNAME_M"
    CASA_PACKAGES=(
        "${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-Gateway/releases/download/v0.4.9-alpha4/linux-${TARGET_ARCH}-casaos-gateway-v0.4.9-alpha4.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-MessageBus/releases/download/v0.4.4-3-alpha2/linux-${TARGET_ARCH}-casaos-message-bus-v0.4.4-3-alpha2.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-UserService/releases/download/v0.4.8/linux-${TARGET_ARCH}-casaos-user-service-v0.4.8.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-LocalStorage/releases/download/v0.4.4/linux-${TARGET_ARCH}-casaos-local-storage-v0.4.4.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}anonimo18032000/CasaOS-AppManagement/archive/refs/tags/v0.4.16-alpha1.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS/releases/download/v0.4.15/linux-${TARGET_ARCH}-casaos-v0.4.15.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-CLI/releases/download/v0.4.4-3-alpha1/linux-${TARGET_ARCH}-casaos-cli-v0.4.4-3-alpha1.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-UI/releases/download/v0.4.20/linux-all-casaos-v0.4.20.tar.gz"
"${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/CasaOS-AppStore/releases/download/v0.4.5/linux-all-appstore-v0.4.5.tar.gz" 
    )
}

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
        Show 1 "Abortado, o OpenWrt não pode ser instalado usando este script."
        exit 1
        ;;
    *alpine*)
        Show 1 "Abortada, a instalação do Alpine ainda não é suportada."
        exit 1
        ;;
    *trisquel*) ;;

    *)
        sType=3
        notice="Não o testamos neste sistema e pode falhar na instalação."
        ;;
    esac
    Show ${sType} "Sua distribuição Linux é : ${DIST} ${notice}"

    if [[ ${sType} == 1 ]]; then
        select yn in "Yes" "No"; do
            case $yn in
            [yY][eE][sS] | [yY])
                Show 0 "A verificação de distribuição foi ignorada."
                break
                ;;
            [nN][oO] | [nN])
                Show 1 "Já saí da instalação."
                exit 1
                ;;
            esac
        done < /dev/tty # < /dev/tty is used to read the input from the terminal
    fi
}

# 3 Check OS
Check_OS() {
    if [[ $UNAME_U == *Linux* ]]; then
        Show 0 "Seu sistema é : $UNAME_U"
    else
        Show 1 "Este script é apenas para Linux."
        exit 1
    fi
}

# 4 Check Memory
Check_Memory() {
    if [[ "${PHYSICAL_MEMORY}" -lt "${MINIMUM_MEMORY}" ]]; then
        Show 1 "requer pelo menos 400 MB de memória física."
        exit 1
    fi
    Show 0 "A verificação da capacidade da memória foi aprovada."
}

# 5 Check Disk
Check_Disk() {
    if [[ "${FREE_DISK_GB}" -lt "${MINIMUM_DISK_SIZE_GB}" ]]; then
        echo -e "${aCOLOUR[4]}O espaço livre em disco recomendado é maior que ${MINIMUM_DISK_SIZE_GB}GB, O espaço livre em disco atual é ${aCOLOUR[3]}${FREE_DISK_GB}GB${COLOUR_RESET}${aCOLOUR[4]}.\nContinuar a instalação??${COLOUR_RESET}"
        select yn in "Yes" "No"; do
            case $yn in
            [yY][eE][sS] | [yY])
                Show 0 "A verificação da capacidade do disco foi ignorada."
                break
                ;;
            [nN][oO] | [nN])
                Show 1 "Já saí da instalação."
                exit 1
                ;;
            esac
        done < /dev/tty  # < /dev/tty is used to read the input from the terminal
    else
        Show 0 "A verificação da capacidade do disco foi aprovada."
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
    Show 2 "Atualizando o gerenciador de pacotes..."
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
    Show 0 "Atualização do gerenciador de pacotes concluída."
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
            Show 1 "Dependência \e[33m$packagesNeeded \e[0m a instalação falhou, tente novamente manualmente!"
            exit 1
        fi
    done
}

# Check Docker running
Check_Docker_Running() {
    for ((i = 1; i <= 3; i++)); do
        sleep 3
        if [[ ! $(${sudo_cmd} systemctl is-active docker) == "active" ]]; then
            Show 1 "Docker não está rodando, tente iniciar"
            ${sudo_cmd} systemctl start docker
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
            Show 1 "A versão mínima recomendada do Docker és \e[33m${MINIMUM_DOCKER_VERSION}.xx.xx\e[0m,\A versão atual do Docker é \e[33m${Docker_Version}\e[0m,\nDesinstale o Docker atual e execute novamente o script de instalação do CasaOS."
            exit 1
        else
            Show 0 "A versão atual do Docker é ${Docker_Version}."
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
            Show 1 "A versão mínima recomendada do Docker é \e[33m${MINIMUM_DOCKER_VERSION}.xx.xx\e[0m,\A versão atual do Docker é \e[33m${Docker_Version}\e[0m,\nDesinstale o Docker atual e execute novamente o script de instalação do CasaOS."
            exit 1
        else
            Show 0 "A versão atual do Docker é ${Docker_Version}."
            Check_Docker_Running
        fi
    else
        Show 1 "Falha na instalação, execute 'curl -fsSL http://get.docker.com | bash' e execute novamente o script de instalação do CasaOS."
        exit 1
    fi
}

#Install Docker
Install_Docker() {
    Show 2 "Instale as dependências necessárias: \e[33mDocker \e[0m"
    if [[ ! -d "${PREFIX}/etc/apt/sources.list.d" ]]; then
        ${sudo_cmd} mkdir -p "${PREFIX}/etc/apt/sources.list.d"
    fi
    GreyStart
    if [[ "${REGION}" = "China" ]] || [[ "${REGION}" = "CN" ]]; then
        ${sudo_cmd} curl -fsSL https://play.cuse.eu.org/get_docker.sh | bash -s docker --mirror Aliyun
    else
        ${sudo_cmd} curl -fsSL https://get.docker.com | bash
    fi
    ColorReset
    if [[ $? -ne 0 ]]; then
        Show 1 "Falha na instalação, tente novamente."
        exit 1
    else
        Check_Docker_Install_Final
    fi
}

#Install Rclone
Install_rclone_from_source() {
  ${sudo_cmd} wget -qO ./install.sh https://rclone.org/install.sh
  if [[ "${REGION}" = "China" ]] || [[ "${REGION}" = "CN" ]]; then
    sed -i 's/downloads.rclone.org/casaos.oss-cn-shanghai.aliyuncs.com/g' ./install.sh
  else
    sed -i 's/downloads.rclone.org/get.casaos.io/g' ./install.sh
  fi
  ${sudo_cmd} chmod +x ./install.sh
  ${sudo_cmd} ./install.sh || {
    Show 1 "Falha na instalação, tente novamente."
    ${sudo_cmd} rm -rf install.sh
    exit 1
  }
  ${sudo_cmd} rm -rf install.sh
  Show 0 "Rclone v1.61.1 instalado com sucesso."
}

Install_Rclone() {
  Show 2 "Instale as dependências necessárias: Rclone"
  if [[ -x "$(command -v rclone)" ]]; then
    version=$(rclone --version 2>>errors | head -n 1)
    target_version="rclone v1.61.1"
    rclone1="${PREFIX}/usr/share/man/man1/rclone.1.gz"
    if [ "$version" != "$target_version" ]; then
      Show 3 "Irá mudar o rclone de $version para $target_version."
      rclone_path=$(command -v rclone)
      ${sudo_cmd} rm -rf "${rclone_path}"
      if [[ -f "$rclone1" ]]; then
        ${sudo_cmd} rm -rf "$rclone1"
      fi
      Install_rclone_from_source
    else
      Show 2 "Versão de destino já instalada."
    fi
  else
    Install_rclone_from_source
  fi
  ${sudo_cmd} systemctl enable rclone || Show 3 "O serviço rclone não existe."
}

#Configuration Addons
Configuration_Addons() {
    Show 2 "Configuração de complementos CasaOS"
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
        mkdir -p ${TMP_ROOT} || Show 1 "Falha ao criar diretório temporário"
        TMP_DIR=$(${sudo_cmd} mktemp -d -p ${TMP_ROOT} || Show 1 "Falha ao criar diretório temporário")

        pushd "${TMP_DIR}"

        for PACKAGE in "${CASA_PACKAGES[@]}"; do
            Show 2 "Baixando ${PACKAGE}..."
            GreyStart
            ${sudo_cmd} wget -t 3 -q --show-progress -c  "${PACKAGE}" || Show 1 "Falha ao baixar o pacote"
            ColorReset
        done

        for PACKAGE_FILE in linux-*.tar.gz; do
            Show 2 "Extraindo ${PACKAGE_FILE}..."
            GreyStart
            ${sudo_cmd} tar zxf "${PACKAGE_FILE}" || Show 1 "Falha ao extrair o pacote"
            ColorReset
        done

        BUILD_DIR=$(${sudo_cmd} realpath -e "${TMP_DIR}"/build || Show 1 "Falha ao encontrar o diretório de compilação")

        popd
    fi

    for SERVICE in "${CASA_SERVICES[@]}"; do
        if ${sudo_cmd} systemctl --quiet is-active "${SERVICE}"; then
            Show 2 "Parando ${SERVICE}..."
            GreyStart
            ${sudo_cmd} systemctl stop "${SERVICE}" || Show 3 "Serviço ${SERVICE} não existe."
            ColorReset
        fi
    done


    Show 2 "Instalando CasaOS..."
    SYSROOT_DIR=$(realpath -e "${BUILD_DIR}"/sysroot || Show 1 "Falha ao encontrar o diretório sysroot")

    # Generate manifest for uninstallation
    MANIFEST_FILE=${BUILD_DIR}/sysroot/var/lib/casaos/manifest
    ${sudo_cmd} touch "${MANIFEST_FILE}" || Show 1 "Falha ao criar arquivo de manifesto"

    GreyStart
    find "${SYSROOT_DIR}" -type f | ${sudo_cmd} cut -c ${#SYSROOT_DIR}- | ${sudo_cmd} cut -c 2- | ${sudo_cmd} tee "${MANIFEST_FILE}" >/dev/null || Show 1 "Falha ao criar arquivo de manifesto"

    ${sudo_cmd} cp -rf "${SYSROOT_DIR}"/* / || Show 1 "Falha ao instalar CasaOS"
    ColorReset

    SETUP_SCRIPT_DIR=$(realpath -e "${BUILD_DIR}"/scripts/setup/script.d || Show 1 "Falha ao encontrar o diretório do script de configuração")

    for SETUP_SCRIPT in "${SETUP_SCRIPT_DIR}"/*.sh; do
        Show 2 "Correndo ${SETUP_SCRIPT}..."
        GreyStart
        ${sudo_cmd} bash "${SETUP_SCRIPT}" || Show 1 "Falha ao executar o script de configuração"
        ColorReset
    done
    
    UI_EVENTS_REG_SCRIPT=/etc/casaos/start.d/register-ui-events.sh
    if [[ -f ${UI_EVENTS_REG_SCRIPT} ]]; then
        ${sudo_cmd} chmod +x $UI_EVENTS_REG_SCRIPT
    fi
    
    # Modify app store configuration
    sed -i "s#https://github.com/IceWhaleTech/_appstore/#${CASA_DOWNLOAD_DOMAIN}IceWhaleTech/_appstore/#g" "$PREFIX/etc/casaos/app-management.conf"

    #Download Uninstall Script
    if [[ -f $PREFIX/tmp/casaos-uninstall ]]; then
        ${sudo_cmd} rm -rf "$PREFIX/tmp/casaos-uninstall"
    fi
    ${sudo_cmd} curl -fsSLk "$CASA_UNINSTALL_URL" >"$PREFIX/tmp/casaos-uninstall"
    ${sudo_cmd} cp -rf "$PREFIX/tmp/casaos-uninstall" $CASA_UNINSTALL_PATH || {
        Show 1 "Falha no download do script de desinstalação. Verifique se sua conexão com a Internet está funcionando e tente novamente."
        exit 1
    }

    ${sudo_cmd} chmod +x $CASA_UNINSTALL_PATH
    
    Install_Rclone

    for SERVICE in "${CASA_SERVICES[@]}"; do
        Show 2 "Começando ${SERVICE}..."
        GreyStart
        ${sudo_cmd} systemctl start "${SERVICE}" || Show 3 "Serviço ${SERVICE} não existe."
        ColorReset
    done
}

Clean_Temp_Files() {
    Show 2 "Limpe arquivos temporários..."
    ${sudo_cmd} rm -rf "${TMP_DIR}" || Show 1 "Falha ao limpar arquivos temporários"
}

Check_Service_status() {
    for SERVICE in "${CASA_SERVICES[@]}"; do
        Show 2 "Verificando ${SERVICE}..."
        if [[ $(${sudo_cmd} systemctl is-active "${SERVICE}") == "active" ]]; then
            Show 0 "${SERVICE} está correndo."
        else
            Show 1 "${SERVICE} não está em execução, reinstale."
            exit 1
        fi
    done
}

# Get the physical NIC IP
Get_IPs() {
    PORT=$(${sudo_cmd} cat ${CASA_CONF_PATH} | grep port | sed 's/port=//')
    ALL_NIC=$($sudo_cmd ls /sys/class/net/ | grep -v "$(ls /sys/devices/virtual/net/)")
    for NIC in ${ALL_NIC}; do
        IP=$($sudo_cmd ifconfig "${NIC}" | grep inet | grep -v 127.0.0.1 | grep -v inet6 | awk '{print $2}' | sed -e 's/addr://g')
        if [[ -n $IP ]]; then
            if [[ "$PORT" -eq "80" ]]; then
                echo -e "${GREEN_BULLET} http://$IP (${NIC})"
            else
                echo -e "${GREEN_BULLET} http://$IP:$PORT (${NIC})"
            fi
        fi
    done
}

# Show Welcome Banner
Welcome_Banner() {
    CASA_TAG=$(casaos -v)

    echo -e "${GREEN_LINE}${aCOLOUR[1]}"
    echo -e " CasaOS ${CASA_TAG}${COLOUR_RESET} está funcionando em${COLOUR_RESET}${GREEN_SEPARATOR}"
    echo -e "${GREEN_LINE}"
    Get_IPs
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

# Step 0 : Get Download Url Domain
Get_Download_Url_Domain
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


# Step 7: Configuration Addon
Configuration_Addons

# Step 8: Download And Install CasaOS
DownloadAndInstallCasaOS

# Step 9: Check Service Status
Check_Service_status

# Step 10: Clear Term and Show Welcome Banner
Welcome_Banner

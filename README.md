
# CasaOS - Sua Nuvem Pessoal
<!-- Readme i18n links -->
<!-- > English | [中文](#) | [Français](#) -->

<p align="center">
    <!-- CasaOS Banner -->
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/IceWhaleTech/logo/main/casaos/casaos_banner_dark_night_800x300.png">
        <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/IceWhaleTech/logo/main/casaos/casaos_banner_twilight_blue_800x300.png">
        <img alt="CasaOS" src="https://raw.githubusercontent.com/IceWhaleTech/logo/main/casaos/casaos_banner_twilight_blue_800x300.png">
    </picture>
    <br/>

### Compatibilidade de hardware

- amd64 / x86-64

### Compatibilidade do sistema

Suporte Oficial
- Debian 12 (✅ Testado, Recomendado)
- Ubuntu Server 20.04 (✅ Testado)

### Configuração rápida CasaOS

Instale um sistema da lista acima e execute este comando:

```sh
wget -qO- https://get.casaos.io | sudo bash
```

### Atualizar CasaOS

O CasaOS pode ser atualizado a partir da interface do usuário (IU), via `Settings ... Update`.  

Alternatively it can be updated from a terminal session.  To update from a terminal session, it must be done either from a secure shell (ssh) session to the device or from a directly attached terminal and keyboard to the device running CasaOS, this cannot be done from the terminal via the CasaOS User Interface (UI).  To update to the latest release of CasaOS from a terminal session run this command:

```sh
wget -qO- https://get.casaos.io/update | sudo bash
```

or

```sh
curl -fsSL https://get.casaos.io/update | sudo bash
```

To determine version of CasaOS from a terminal session run this command:

```sh
casaos -v
```



### Uninstall CasaOS


v0.3.3 or newer

```sh
casaos-uninstall
```

Before v0.3.3

```sh
curl -fsSL https://get.icewhale.io/casaos-uninstall.sh | sudo bash
```

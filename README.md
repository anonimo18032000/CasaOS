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
    <i>Conecte-se com a comunidade, estabeleça autonomia, reduza o custo de SaaS e MAXIMIZE o potencial de um copiloto personalizado.</i>
    <br/>
    <br/>
    <!-- Fork Badges (point at anonimo18032000/CasaOS, not upstream) -->
    <a href="https://github.com/anonimo18032000/CasaOS/releases" target="_blank">
        <img alt="Fork Version" src="https://img.shields.io/github/v/release/anonimo18032000/CasaOS?include_prereleases&color=162453&style=flat-square&label=Fork" />
    </a>
    <a href="https://github.com/anonimo18032000/CasaOS/blob/main/LICENSE" target="_blank">
        <img alt="CasaOS License" src="https://img.shields.io/github/license/anonimo18032000/CasaOS?color=162453&style=flat-square&label=License" />
    </a>
    <a href="https://github.com/anonimo18032000/CasaOS/pulls" target="_blank">
        <img alt="Fork Pull Requests" src="https://img.shields.io/github/issues-pr/anonimo18032000/CasaOS?color=162453&style=flat-square&label=PRs" />
    </a>
    <a href="https://github.com/anonimo18032000/CasaOS/stargazers" target="_blank">
        <img alt="Fork Stargazers" src="https://img.shields.io/github/stars/anonimo18032000/CasaOS?color=162453&style=flat-square&label=Stars" />
    </a>
    <br/>
    <!-- Upstream project badges, for reference -->
    <a href="https://github.com/IceWhaleTech/CasaOS/issues" target="_blank">
        <img alt="Upstream Issues (issues are disabled on this fork; report upstream bugs there)" src="https://img.shields.io/github/issues/IceWhaleTech/CasaOS?color=162453&style=flat-square&label=Upstream%20Issues" />
    </a>
    <a href="https://github.com/IceWhaleTech/CasaOS/discussions" target="_blank">
        <img alt="CasaOS GitHub Discussions" src="https://img.shields.io/github/discussions/IceWhaleTech/CasaOS?color=162453&style=flat-square&label=Discussions&logo=github" />
    </a>
<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
    <a href="#créditos">
        <img alt="All Contributors" src="https://img.shields.io/static/v1?label=All%20Contributors&message=15&color=162453&style=flat-square&logo=Handshake&logoColor=fff" />
    </a>
    <br/>
    <!-- CasaOS Links -->
    <a href="https://github.com/anonimo18032000/CasaOS" target="_blank">Este Fork</a> |
    <a href="https://github.com/IceWhaleTech/CasaOS" target="_blank">GitHub Upstream</a>
    <br/>
    <br/>
    <!-- CasaOS Snapshots -->
    <kbd>
      <picture>
          <source media="(prefers-color-scheme: dark)" srcset="snapshot-dark.jpg">
          <source media="(prefers-color-scheme: light)" srcset="snapshot-light.jpg">
          <img alt="CasaOS Snapshot" src="snapshot-light.jpg">
      </picture>
    </kbd>
</p>

## Sobre Este Fork

Este é um fork modificado do [IceWhaleTech/CasaOS](https://github.com/IceWhaleTech/CasaOS) mantido por [anonimo18032000](https://github.com/anonimo18032000). Ele adiciona várias funcionalidades e correções de bugs em cima do CasaOS upstream `v0.4.15`:

- **Armazenamento remoto SFTP**: conecte/edite montagens SFTP a partir da interface (host, porta, usuário/senha ou chave privada, nome de exibição personalizado, pasta raiz remota), com um botão de "testar conexão" antes de montar.
- **Painel de saúde do armazenamento**: status/latência de cada montagem de rede, além de reconexão com um clique.
- **Backups agendados**: crie tarefas de backup baseadas em cron que sincronizam uma pasta local com qualquer remoto configurado (SFTP/Dropbox/GDrive/OneDrive), com execução imediata, edição e exclusão.
- **Atualizações automáticas de apps (opcionais)**: verificação a cada hora que atualiza os apps Compose instalados quando há uma nova imagem disponível.
- **Taxa de atualização do painel configurável**: escolha o intervalo de consulta do status de hardware (250ms/500ms/1s/2s/5s) em vez de um valor fixo de 5s.
- **Suporte a HTTPS** (no [fork do CasaOS-Gateway](https://github.com/anonimo18032000/CasaOS-Gateway)): geração de certificado autoassinado ou upload de certificado próprio, com redirecionamento automático de HTTP → HTTPS.
- **Feedback de instalação**: notificações toast no início/sucesso/erro da instalação de apps, em vez de uma instalação silenciosa em segundo plano.
- **Atualização segura**: o botão `Configurações ... Atualizar` (e o `UpdateUrl` vazio que ele usava) agora aponta por padrão para o instalador deste fork em vez de `get.casaos.io`, então clicar em Atualizar não sobrescreve mais silenciosamente as funcionalidades do fork com o CasaOS oficial.
- **Aviso de "atualização disponível" preciso**: a checagem de versão agora consulta as releases deste fork no GitHub em vez da API do upstream (`api.casaos.io`), então o indicador nas Configurações reflete o cronograma de releases do fork, não o do CasaOS oficial.
- Diversas correções de bugs: upload de chave SFTP rejeitando chaves sem extensão (`id_rsa`), um bug de parsing do nome de armazenamento que quebrava remontagens com um caminho remoto customizado, um limite de 1 segundo do `robfig/cron` que ignorava silenciosamente taxas de atualização sub-segundo, e um bug de exibição "NaN undefined/s" na velocidade de rede em taxas de atualização rápidas.

Este fork também tem forks complementares para os outros componentes afetados: [CasaOS-UI](https://github.com/anonimo18032000/CasaOS-UI), [CasaOS-AppManagement](https://github.com/anonimo18032000/CasaOS-AppManagement) e [CasaOS-Gateway](https://github.com/anonimo18032000/CasaOS-Gateway). Todos os demais componentes (MessageBus, UserService, LocalStorage, CLI, AppStore) não foram modificados e vêm direto das releases oficiais do upstream.

### Instalar este fork

```sh
curl -fsSL https://raw.githubusercontent.com/anonimo18032000/CasaOS/main/install.sh | sudo bash
```

O instalador funciona da mesma forma que o comando oficial abaixo, exceto que baixa o CasaOS, CasaOS-UI, CasaOS-AppManagement e CasaOS-Gateway das [releases](https://github.com/anonimo18032000/CasaOS/releases) deste fork em vez do upstream. Os binários do fork foram compilados apenas para **amd64**; em outras arquiteturas, o script automaticamente usa os builds oficiais do upstream para esses três componentes.

> ⚠️ Este é um fork não oficial, modificado pela comunidade, sem afiliação ou suporte da IceWhaleTech. Para o projeto original, veja abaixo.

---

## Por que você precisa de uma Nuvem Pessoal?

Em 2020, a equipe notou três tendências importantes:
- O custo de poder computacional e armazenamento estava caindo rapidamente.
- Uma parte da computação em nuvem estava migrando para a computação de borda (edge computing).
- A questão da propriedade e atribuição dos ativos de dados dos consumidores vinha sendo ignorada.

Com base nessas tendências, a equipe propôs internamente um experimento mental: e se nuvens pessoais estivessem disponíveis por menos de US$ 100 nos próximos cinco anos? Essa nuvem pessoal forneceria uma solução de colaboração de dados de baixo custo, funcionando como um data center pessoal, armazenando e gerenciando dados para criadores e pequenas organizações. Uma rede de computação colaborativa distribuída poderia ser formada por servidores pessoais localizados em todo o mundo. Ela também poderia controlar e conectar todos os dispositivos inteligentes, fornecendo serviços inteligentes locais entre ecossistemas.

Além disso, a nuvem pessoal poderia combinar dados pessoais para treinar assistentes de IA personalizados. A ideia é que essa tecnologia seria uma forma eficaz de resolver a questão da propriedade dos ativos de dados dos consumidores, além de fornecer uma solução de computação mais acessível e eficiente para indivíduos e pequenas organizações.

> Se você acha que o que estamos fazendo é valioso, por favor **nos dê uma estrela ⭐** e **faça um fork 🤞**!

## Funcionalidades

- Interface amigável projetada para cenários domésticos
  - Sem código, sem formulários, intuitiva, projetada para humanos
- Suporte a múltiplos hardwares e sistemas base
  - ZimaBoard, NUC, RPi, computadores antigos, o que estiver disponível.
- Apps selecionados na loja de aplicativos, instalação com um clique
  - Nextcloud, HomeAssistant, AdGuard, Jellyfin, *arr e muito mais!
- Instale facilmente inúmeros apps Docker
  - Mais de 100.000 apps do ecossistema Docker podem ser instalados facilmente
- Gerenciamento elegante de discos e arquivos
  - O que você vê é o que você tem. Nenhum conhecimento técnico necessário.
- Widgets de sistema/apps bem projetados
  - O que importa para você, em um piscar de olhos. Uso de recursos, status dos apps e muito mais!

## Primeiros Passos

O CasaOS suporta totalmente ZimaBoard, Intel NUC e Raspberry Pi. Além disso, é totalmente compatível com outros computadores e placas de desenvolvimento, com Ubuntu, Debian, Raspberry Pi OS e CentOS, através de instalação com um único comando.

### Compatibilidade de Hardware

- amd64 / x86-64
- arm64
- armv7

### Compatibilidade de Sistema

Suporte Oficial
- Debian 12 (✅ Testado, Recomendado)
- Ubuntu Server 20.04 (✅ Testado)
- Raspberry Pi OS (✅ Testado)

Suporte da Comunidade
- Elementary 6.1 (✅ Testado)
- Armbian 22.04 (✅ Testado)
- Alpine (🚧 Ainda não testado completamente)
- OpenWrt (🚧 Ainda não testado completamente)
- ArchLinux (🚧 Ainda não testado completamente)

### Instalação Rápida do CasaOS (build oficial upstream)

Instale um sistema novo a partir da lista acima e execute este comando para instalar o CasaOS **original, sem modificações**, da IceWhaleTech:

```sh
wget -qO- https://get.casaos.io | sudo bash
```

ou

```sh
curl -fsSL https://get.casaos.io | sudo bash
```

> Se você quiser as funcionalidades **deste fork** (montagens SFTP, backups agendados, saúde do armazenamento, HTTPS, etc.), use o comando em [Sobre Este Fork](#sobre-este-fork) em vez deste — não misture os dois na mesma máquina.

### Atualizar o CasaOS

> Se você instalou **este fork**, o botão `Configurações ... Atualizar` da interface já é seguro: por padrão, ele reexecuta o instalador **deste fork** (não o do upstream) — veja `UpdateUrl` em `service/system.go` e no `casaos.conf.sample`. Os comandos de terminal abaixo (`get.casaos.io/update`) continuam sendo os do **CasaOS oficial**; para atualizar o fork pelo terminal, execute de novo o comando de instalação deste fork (veja [Sobre Este Fork](#sobre-este-fork)) em vez dos comandos abaixo.

O CasaOS pode ser atualizado a partir da Interface do Usuário (UI), em `Configurações ... Atualizar`.  

Alternativamente, ele pode ser atualizado a partir de uma sessão de terminal. Para atualizar por uma sessão de terminal, isso deve ser feito por uma sessão segura (ssh) até o dispositivo ou por um terminal e teclado conectados diretamente ao dispositivo que executa o CasaOS — isso não pode ser feito pelo terminal embutido na Interface do Usuário (UI) do CasaOS. Para atualizar para a versão mais recente do CasaOS a partir de uma sessão de terminal, execute este comando:

```sh
wget -qO- https://get.casaos.io/update | sudo bash
```

ou

```sh
curl -fsSL https://get.casaos.io/update | sudo bash
```

Para verificar a versão do CasaOS a partir de uma sessão de terminal, execute este comando:

```sh
casaos -v
```



### Desinstalar o CasaOS


v0.3.3 ou mais recente

```sh
casaos-uninstall
```

Antes da v0.3.3

```sh
curl -fsSL https://get.icewhale.io/casaos-uninstall.sh | sudo bash
```

## Comunidade 

> A seção abaixo descreve o projeto CasaOS upstream e sua comunidade/equipe, não este fork.

A palavra Casa vem do espanhol e significa "lar". O projeto CasaOS surgiu como um sistema pré-instalado no produto financiado via crowdfunding ZimaBoard no Kickstarter.

Depois de analisar muitos sistemas e softwares no mercado, a equipe constatou, infelizmente, que não havia nenhum sistema de servidor projetado para cenários domésticos.

Por isso, decidimos construir este projeto de código aberto para desenvolver o CasaOS com nossas próprias mãos — todos na comunidade, e você também.

Acreditamos que, por meio da inovação colaborativa impulsionada pela comunidade e da comunicação aberta com desenvolvedores do mundo todo, podemos transformar a experiência doméstica digital como nunca antes.

**Você é muito bem-vindo para buscar ajuda ou compartilhar grandes ideias com a comunidade!**

## Contribuindo

O CasaOS é um projeto de código aberto impulsionado pela comunidade, e as pessoas envolvidas são usuários do CasaOS. Isso significa que o CasaOS sempre vai precisar de contribuições de membros da comunidade como você!

## Créditos

> A lista abaixo credita os contribuidores do projeto CasaOS **upstream**. Nenhum deles é responsável, ou necessariamente tem conhecimento, das alterações deste fork — veja [Sobre Este Fork](#sobre-este-fork) para saber o que foi modificado aqui.

Muito obrigado a todos que ajudaram o CasaOS até agora!

A contribuição de todos é muito apreciada.

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tr>
    <td align="center"><a href="https://github.com/jerrykuku"><img src="https://avatars.githubusercontent.com/u/9485680?v=4?s=100" width="100px;" alt=""/><br /><sub><b>老竭力</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=jerrykuku" title="Code">💻</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=jerrykuku" title="Documentation">📖</a> <a href="#ideas-jerrykuku" title="Ideas, Planning, & Feedback">🤔</a> <a href="#infra-jerrykuku" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a> <a href="#maintenance-jerrykuku" title="Maintenance">🚧</a> <a href="#platform-jerrykuku" title="Packaging/porting to new platform">📦</a> <a href="#question-jerrykuku" title="Answering Questions">💬</a> <a href="https://github.com/IceWhaleTech/CasaOS/pulls?q=is%3Apr+reviewed-by%3Ajerrykuku" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/LinkLeong"><img src="https://avatars.githubusercontent.com/u/13556972?v=4?s=100" width="100px;" alt=""/><br /><sub><b>link</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=LinkLeong" title="Code">💻</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=LinkLeong" title="Documentation">📖</a> <a href="#ideas-LinkLeong" title="Ideas, Planning, & Feedback">🤔</a> <a href="#infra-LinkLeong" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a> <a href="#maintenance-LinkLeong" title="Maintenance">🚧</a> <a href="#question-LinkLeong" title="Answering Questions">💬</a> <a href="https://github.com/IceWhaleTech/CasaOS/pulls?q=is%3Apr+reviewed-by%3ALinkLeong" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/tigerinus"><img src="https://avatars.githubusercontent.com/u/7172560?v=4?s=100" width="100px;" alt=""/><br /><sub><b>太戈</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=tigerinus" title="Code">💻</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=tigerinus" title="Documentation">📖</a> <a href="#ideas-tigerinus" title="Ideas, Planning, & Feedback">🤔</a> <a href="#infra-tigerinus" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a> <a href="#maintenance-tigerinus" title="Maintenance">🚧</a> <a href="#mentoring-tigerinus" title="Mentoring">🧑‍🏫</a> <a href="#security-tigerinus" title="Security">🛡️</a> <a href="#question-tigerinus" title="Answering Questions">💬</a> <a href="https://github.com/IceWhaleTech/CasaOS/pulls?q=is%3Apr+reviewed-by%3Atigerinus" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://github.com/Lauren-ED209"><img src="https://avatars.githubusercontent.com/u/8243355?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Lauren</b></sub></a><br /><a href="#ideas-Lauren-ED209" title="Ideas, Planning, & Feedback">🤔</a> <a href="#fundingFinding-Lauren-ED209" title="Funding Finding">🔍</a> <a href="#projectManagement-Lauren-ED209" title="Project Management">📆</a> <a href="#question-Lauren-ED209" title="Answering Questions">💬</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=Lauren-ED209" title="Tests">⚠️</a></td>
    <td align="center"><a href="https://JohnGuan.Cn"><img src="https://avatars.githubusercontent.com/u/3358477?v=4?s=100" width="100px;" alt=""/><br /><sub><b>John Guan</b></sub></a><br /><a href="#blog-JohnGuan" title="Blogposts">📝</a> <a href="#content-JohnGuan" title="Content">🖋</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=JohnGuan" title="Documentation">📖</a> <a href="#ideas-JohnGuan" title="Ideas, Planning, & Feedback">🤔</a> <a href="#eventOrganizing-JohnGuan" title="Event Organizing">📋</a> <a href="#mentoring-JohnGuan" title="Mentoring">🧑‍🏫</a> <a href="#question-JohnGuan" title="Answering Questions">💬</a> <a href="https://github.com/IceWhaleTech/CasaOS/pulls?q=is%3Apr+reviewed-by%3AJohnGuan" title="Reviewed Pull Requests">👀</a></td>
    <td align="center"><a href="https://blog.tippybits.com"><img src="https://avatars.githubusercontent.com/u/17506770?v=4?s=100" width="100px;" alt=""/><br /><sub><b>David Tippett</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=dtaivpp" title="Documentation">📖</a> <a href="#ideas-dtaivpp" title="Ideas, Planning, & Feedback">🤔</a> <a href="#question-dtaivpp" title="Answering Questions">💬</a></td>
    <td align="center"><a href="https://github.com/zarevskaya"><img src="https://avatars.githubusercontent.com/u/60230221?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Skaya</b></sub></a><br /><a href="#mentoring-zarevskaya" title="Mentoring">🧑‍🏫</a> <a href="#question-zarevskaya" title="Answering Questions">💬</a> <a href="#tutorial-zarevskaya" title="Tutorials">✅</a> <a href="#translation-zarevskaya" title="Translation">🌍</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/AuthorShin"><img src="https://avatars.githubusercontent.com/u/4959043?v=4?s=100" width="100px;" alt=""/><br /><sub><b>AuthorShin</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=AuthorShin" title="Tests">⚠️</a> <a href="https://github.com/IceWhaleTech/CasaOS/issues?q=author%3AAuthorShin" title="Bug reports">🐛</a> <a href="#question-AuthorShin" title="Answering Questions">💬</a> <a href="#ideas-AuthorShin" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/baptiste313"><img src="https://avatars.githubusercontent.com/u/93325157?v=4?s=100" width="100px;" alt=""/><br /><sub><b>baptiste313</b></sub></a><br /><a href="#translation-baptiste313" title="Translation">🌍</a></td>
    <td align="center"><a href="https://github.com/DrMxrcy"><img src="https://avatars.githubusercontent.com/u/58747968?v=4?s=100" width="100px;" alt=""/><br /><sub><b>DrMxrcy</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=DrMxrcy" title="Tests">⚠️</a> <a href="#ideas-DrMxrcy" title="Ideas, Planning, & Feedback">🤔</a> <a href="#question-DrMxrcy" title="Answering Questions">💬</a></td>
    <td align="center"><a href="https://github.com/Joooost"><img src="https://avatars.githubusercontent.com/u/12090673?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Joooost</b></sub></a><br /><a href="#ideas-Joooost" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://potyarkin.ml"><img src="https://avatars.githubusercontent.com/u/334908?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Vitaly Potyarkin</b></sub></a><br /><a href="#ideas-sio" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/bearfrieze"><img src="https://avatars.githubusercontent.com/u/1023813?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Bjørn Friese</b></sub></a><br /><a href="#ideas-bearfrieze" title="Ideas, Planning, & Feedback">🤔</a></td>
    <td align="center"><a href="https://github.com/Protektor-Desura"><img src="https://avatars.githubusercontent.com/u/1195496?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Protektor</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/issues?q=author%3AProtektor-Desura" title="Bug reports">🐛</a> <a href="#ideas-Protektor-Desura" title="Ideas, Planning, & Feedback">🤔</a> <a href="#question-Protektor-Desura" title="Answering Questions">💬</a></td>
  </tr>
  <tr>
    <td align="center"><a href="https://github.com/llwaini"><img src="https://avatars.githubusercontent.com/u/59589857?v=4?s=100" width="100px;" alt=""/><br /><sub><b>llwaini</b></sub></a><br /><a href="#projectManagement-llwaini" title="Project Management">📆</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=llwaini" title="Tests">⚠️</a> <a href="#tutorial-llwaini" title="Tutorials">✅</a></td>
    <td align="center"><a href="https://github.com/CorrectRoadH"><img src="https://avatars.githubusercontent.com/u/29306285?v=4?s=100" width="100px;" alt=""/><br /><sub><b>CorrectRoadH</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=correctroadh" title="Code">💻</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=correctroadh" title="Documentation">📖</a></td>
    <td align="center"><a href="https://github.com/zhanghengxin"><img src="https://avatars.githubusercontent.com/u/24197448?v=4?s=100" width="100px;" alt=""/><br /><sub><b>zhanghengxin</b></sub></a><br /><a href="https://github.com/IceWhaleTech/CasaOS/commits?author=zhanghengxin" title="Code">💻</a> <a href="https://github.com/IceWhaleTech/CasaOS/commits?author=zhanghengxin" title="Documentation">📖</a></td>
  </tr>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

Este projeto segue a especificação [all-contributors](https://github.com/all-contributors/all-contributors). Contribuições de qualquer tipo são bem-vindas!

## Changelog

As alterações feitas **neste fork** estão documentadas em suas próprias [notas de versão](https://github.com/anonimo18032000/CasaOS/releases). Para o histórico do CasaOS upstream, veja as [notas de versão oficiais](https://github.com/IceWhaleTech/CasaOS/releases).

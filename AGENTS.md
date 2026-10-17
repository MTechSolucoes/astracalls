# AGENTS.md — AstraCalls

Guia de rotina para quem (pessoa ou agente) for buildar, publicar e implantar o
AstraCalls. Foco na **build da imagem Docker** e no **deploy**.

## Arquitetura da imagem: amd64 (arm64 opcional)

A imagem `mtechsolucoes/callfy` é publicada hoje só para **`linux/amd64`** — todos
os nós do Swarm são x86. O `Dockerfile` é arch-aware (compila o codec MLow com
NEON no ARM e baseline SSE2 no x86), então voltar a publicar `linux/arm64` é só
religar o job ARM no CI (ver abaixo) quando entrar um servidor ARM.

## Como buildar: via CI (nunca na VPS de produção)

O build é feito **na CI**, no runner x86 do GitHub (`ubuntu-latest`). O job
publica por **digest** e um job `merge` monta o **manifest list** nas tags
finais. Workflow: [`.github/workflows/docker-multiarch.yml`](.github/workflows/docker-multiarch.yml).

Para reativar o arm64: conectar o repositório ao **Ubicloud** (GitHub Actions
runners) e voltar a entrada `linux/arm64` / `ubicloud-standard-8-arm` na matrix
do workflow. Sem o Ubicloud conectado esse job fica preso na fila e a imagem
não é publicada.

### Gatilhos

| Evento | Resultado |
|--------|-----------|
| push em `develop` | publica `:develop` |
| tag `v*` | publica `:vX.Y.Z` + `:latest` |
| manual (`workflow_dispatch`) | roda sob demanda |

Ou seja: **para gerar imagem nova, basta dar push** (ou criar a tag de versão).
Não é necessário — nem recomendado — buildar imagem na VPS.

### Pré-requisitos (configurar uma vez)

- GitHub Actions habilitado no repositório (é um fork: vem desligado).
- Secrets do repositório: `DOCKERHUB_USERNAME` e `DOCKERHUB_TOKEN`.
- O checkout usa `submodules: recursive` (o `opus_mlow` é submódulo).

### Build local (teste)

Para uma imagem de teste fora do CI (ex.: no WSL), inicialize o submódulo e
builde uma arquitetura:

```bash
git submodule update --init --recursive
docker buildx build --platform linux/amd64 -t mtechsolucoes/callfy:test --load .
```

Nunca builde na VPS de produção: o build compete por disco e CPU com o Postgres.

## Deploy (Docker Swarm / Portainer)

```bash
docker service update --with-registry-auth   --image mtechsolucoes/callfy:develop callfy_callfy
```

Pelo Portainer: atualize o serviço com "Re-pull image" ligado. Sempre verifique
o painel após o deploy.

## Versionamento

Não corte versão (tag `v*` / release / `CHANGELOG`) sem pedido explícito.
Correções na `develop` são livres. Atualize o `CHANGELOG.md` ao cortar release.

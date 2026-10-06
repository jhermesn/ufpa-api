# Resultados da validação

Validação ponta a ponta feita em 06/10/2026, numa conta de testes em `us-east-1`, antes da palestra de 16/10/2026.

Ambiente: macOS (Apple Silicon), OrbStack com Docker 29.4.0 (containerd image store), buildx 0.33.0, AWS CLI 2.34.53, Trivy 0.71.1, cfn-lint e checkov locais.

## Números para os slides

| Métrica | Valor medido |
|---|---|
| Imagem ingênua (`golang:1.27`), `docker images` | **1,44 GB** (337 MB comprimida) |
| Imagem multi-stage (distroless), `docker images` | **16,8 MB** (3,69 MB comprimida; 3,68 MB no ECR) |
| Redução | ~86x no `docker images`, ~91x comprimida |
| CVEs HIGH/CRITICAL na ingênua (Trivy) | **194** (192 HIGH, 2 CRITICAL; 125 CVEs únicos, todos em pacotes do Debian 13.7) |
| CVEs HIGH/CRITICAL na multi-stage (Trivy) | **0** |
| Scan on push do ECR (basic) em v1 e v2 | 0 findings |
| `govulncheck ./...` | Nenhuma vulnerabilidade |
| `make scan` com DB em cache (duas imagens) | 5,5 s |
| `make scan-warmup` (download de 119 MB) | 9 s |
| `make bootstrap` (stack nova, DesiredCount=0) | **3 min 43 s** |
| `make release VERSION=v1` (build + push + deploy) | 2 min 30 s |
| `make build VERSION=v2 && make push VERSION=v2` | 16 s (cache quente) |
| `make deploy VERSION=v2` (até o CloudFormation concluir) | **3 min 50 s** |
| Respostas com falha durante o rollout v1 → v2 | **0 de 285** |
| Janela em que v1 e v2 responderam juntas | ~65 s |
| `make kill-task` até o service voltar a 2 tasks healthy | **~55 s** |
| Respostas com falha durante o self-healing | **0 de 78** |
| `make destroy` | 4 min 8 s |
| Tempo de vida total da stack no teste | ~25 min |
| Custo real no Cost Explorer | Indisponível no dia (dados atrasam até 24 h). Ver abaixo como conferir |

## 1. Premissas verificadas

| Premissa | Resultado | Fonte |
|---|---|---|
| Go 1.25 é a versão certa | **Errada.** Estável atual é 1.27.1 (1.27.0 saiu em 19/08/2026). Go só dá suporte às duas últimas majors, então 1.25 já não recebe correções de segurança. Corrigido para 1.27 | https://go.dev/dl/?mode=json, https://go.dev/doc/devel/release |
| Tags `golang:1.27-alpine`, `golang:1.27` e `gcr.io/distroless/static:nonroot` existem | Existem, todas como index OCI multi-arch com `arm64` | `docker buildx imagetools inspect` nos registries |
| Trust policy da task role com `aws:SourceAccount` + `aws:SourceArn` | Suportada e recomendada. A doc avisa que `aws:SourceArn` por cluster não é suportado, por isso o curinga `arn:aws:ecs:<região>:<conta>:*` (que é o que o template usa) | https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html |
| Bucket de layers `prod-${region}-starport-layer-bucket` | Correto, com `/*` e `s3:GetObject` | https://docs.aws.amazon.com/AmazonECR/latest/userguide/vpc-endpoints.html |
| `EmptyOnDelete` em `AWS::ECR::Repository` | Existe. Confirmado na prática: o `make destroy` removeu o repositório com 2 versões dentro | https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-ecr-repository.html |
| `aws cloudformation deploy` reaproveita parâmetros não informados | Sim: "If you're updating a stack and you don't specify a parameter, the command uses the stack's existing value" | `aws cloudformation deploy help` (CLI 2.34.53) |
| `ReadonlyRootFilesystem` + `Capabilities.Drop: [ALL]` no Fargate ARM64 | Suportado. No Fargate a restrição é só no `add` (apenas `SYS_PTRACE`); `drop` aceita `ALL`. Confirmado na prática: tasks subiram e responderam | https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_KernelCapabilities.html, https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html |
| Flags `--skip-db-update` e `--download-db-only` do Trivy | Existem na 0.71.1 e funcionaram | `trivy image --help` |
| Saída de `imagetools inspect --format '{{json .Manifest.Digest}}'` | Imprime `"sha256:..."` entre aspas e sem quebra de linha; o `tr -d '"'` do script resolve. O digest é o do index multi-arch, que é o certo para fixar | Execução local |

## 2. O que mudou e por quê

| Commit | Mudança | Por quê |
|---|---|---|
| `fix(go)` | Go 1.25 → 1.27 em `go.mod`, nos dois Dockerfiles e no README | 1.25 ficou sem suporte |
| `fix(scripts)` | `pin-digests.sh` reescrito sem `grep -P` e sem `sed -i` | No macOS (grep/sed BSD) o `grep -P` falhava dentro do `for ... in $(...)`, o `set -e` não pegava e o `make pin` saía com código 0 **sem fixar nada**. O `sed -i -E` também quebraria (o BSD lê `-E` como sufixo de backup). Testado: fixa as duas imagens, é idempotente e falha com código 1 se a imagem não existe |
| `chore(docker)` | Digests reais fixados no `Dockerfile` | Resultado do `make pin` |
| `fix(make)` | `make watch` usa `curl -sf` | Sem `--fail`, um 502/503 do ALB saía com código 0 e imprimia o HTML de erro, ou seja, falhas no rollout nunca apareciam como `{"error":...}`. Sem isso a métrica "zero falhas" não valia nada |
| `fix(infra)` | `LoadBalancer` com `DependsOn: InternetGatewayAttachment` | Um ALB internet-facing falha com "VPC has no internet gateway" se o CloudFormation criá-lo antes de o IGW estar anexado; nada no template garantia essa ordem. `cfn-lint` continua limpo e o `checkov` continua com 35 aprovados e 7 pulados |
| `docs(readme)` | Tamanho da imagem, tempos de bootstrap/deploy/self-healing e aviso sobre `make logs` | O README prometia "< 10 MB" e "~5 min", mas no telão vai aparecer 16,8 MB e ~4 min |

O `Dockerfile.naive` não foi fixado por digest de propósito: ele é o exemplo do que não fazer.

## 3. Comportamentos observados (para ensaiar)

- **Tamanho no `docker images`.** Com o containerd image store (padrão do Docker 29 e do OrbStack), a coluna mostra o uso em disco (camadas comprimidas + descompactadas). Por isso aparece 16,8 MB, e não os 3,7 MB do ECR. `docker image ls --tree ufpa-api` mostra as duas colunas, se alguém perguntar.
- **Scan no console do ECR.** O `buildx --load` gera um index OCI com a imagem arm64 e um manifest de attestation. A tag `v1` aponta para o index, e o scan on push roda na imagem arm64 sem tag. No console, procure o resultado na linha sem tag de ~3,7 MB, não na linha `v1`.
- **Rolling deploy.** O ECS sobe os 2 tasks novos (100/200), registra no target group, drena os antigos por 10 s e manda SIGTERM. A app loga `shutting down` e sai com código 0. O CloudFormation leva ~35 s antes de o primeiro task subir e ~35 s depois do "deployment completed" do ECS.
- **Self-healing.** Depois do `stop-task`, o scheduler do ECS levou ~26 s para reagir (deregister + novo task). Nesse intervalo o task parado continua respondendo, e por isso o `make watch` não mostra nenhum erro. O que o público vê: o `task_id` antigo some ~28 s depois do comando, só o task sobrevivente responde por ~20 s, e um `task_id` novo aparece ~50 s depois do comando. Vale narrar isso, ou mostrar o `aws ecs describe-services` / console em paralelo.
- **`make logs`.** O `aws logs tail --follow` mostra só os últimos 10 minutos e os health checks não são logados. Sem tráfego recente a tela fica vazia: deixe o `make watch` rodando antes.

## 4. Custo

O Cost Explorer ainda não tinha dados do dia na hora do teste (`Estimated: true`, sem grupos). Para conferir depois de 24 h:

```bash
aws ce get-cost-and-usage --region us-east-1 \
  --time-period Start=2026-10-06,End=2026-10-07 --granularity DAILY \
  --metrics UnblendedCost --group-by Type=DIMENSION,Key=SERVICE
```

Cada chamada à API do Cost Explorer custa US$ 0,01.

## 5. O que não foi validado

- HTTPS com `CERT_ARN` (precisa de certificado ACM e domínio).
- `make run-local` em x86 (validado só em arm64, com `--read-only --cap-drop ALL`).

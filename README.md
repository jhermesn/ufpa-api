# ufpa-api

Demo da palestra **"Containers na AWS desmistificados: da imagem ao deploy seguro"**, apresentada no lançamento do Student Builder Group UFPA.

É uma API em Go que responde com a identidade do task ECS que atendeu a requisição. Com `curl` em loop dá para ver o load balancing entre AZs, o rolling deploy e o self-healing acontecendo.

```
GET /v1/info  → {"app":"ufpa-api","version":"v1","task_id":"a1b2c3","az":"us-east-1a"}
GET /healthz  → {"status":"ok"}
```

## Arquitetura

```
Internet → ALB (subnets públicas, 2 AZs)
              ↓ :8080 (SG: só do ALB)
        ECS Fargate ARM64, 2 tasks (subnets privadas, sem IP público)
              ↓ egress :443
        NAT Gateway (1 AZ) → ECR API, CloudWatch Logs
        S3 Gateway Endpoint → layers do ECR (gratuito, fora do NAT)
```

| Ponto | Como está | Por quê |
|---|---|---|
| Imagem | Multi-stage, `distroless/static:nonroot`, cross-compile sem QEMU | < 10 MB, sem shell, sem root |
| Runtime | `ReadonlyRootFilesystem`, `Drop: ALL`, UID 65532 | Defesa em profundidade |
| Execution role | Pull só deste repositório, escrita só neste log group | Usada pelo agente ECS |
| Task role | Vazia, com condição contra confused deputy | Usada pela app, que não chama nenhuma API AWS |
| Deploy | Rolling 100/200 + circuit breaker com rollback | Zero downtime e rollback automático |
| Shutdown | App 8s < `StopTimeout` 10s; deregistration 10s | Encerra antes do SIGKILL |
| Keep-alive | App 65s > idle do ALB 60s | Evita 502 em conexões reaproveitadas |
| ECR | Tags imutáveis, scan on push, lifecycle de 10 imagens | Toda versão é rastreável |

## Pré-requisitos

- Docker com buildx
- AWS CLI v2 com sessão temporária (SSO): `aws sso login --profile <perfil>` e `export AWS_PROFILE=<perfil>`
- Trivy
- Go 1.25+ (só para `make test`)

## Véspera do evento

```bash
make pin                       # fixa os digests das imagens base no Dockerfile (commit)
make scan-warmup               # baixa o DB do Trivy (evita travar no palco)
make bootstrap                 # cria infra com DesiredCount=0 (~5 min; repositório ainda vazio)
make release VERSION=v1        # build ARM64 → push → service com 2 tasks
make build VERSION=v2 && make push VERSION=v2   # deixa a v2 pronta no ECR
make watch                     # validar
```

Faça `make build-naive` e `make build` também na véspera para aquecer o cache de layers.

## No palco

| Bloco | Comando | O que mostrar |
|---|---|---|
| Imagem | `make build-naive && make build && make compare` | Diferença de tamanho entre a imagem ingênua e a multi-stage |
| Segurança | `make scan` | Contagem de CVEs HIGH/CRITICAL das duas imagens |
| Load balancing | `make watch` (terminal 2) | `task_id` e `az` alternando |
| Rolling deploy | `make deploy VERSION=v2` | `v1 → v2` sem nenhuma resposta falhar |
| Self-healing | `make kill-task` | O ECS sobe um task novo e o ALB tira o morto do pool |
| Logs | `make logs` | Logs JSON no CloudWatch |

`make deploy VERSION=v2` leva de 2 a 4 min porque o CloudFormation espera o service estabilizar. Use esse tempo para o slide de task role vs execution role.

Para rodar localmente em x86: `make build PLATFORM=linux/amd64 && make run-local`.

## HTTPS (opcional)

```bash
make deploy CERT_ARN=arn:aws:acm:us-east-1:<conta>:certificate/<id>
```

A porta 80 passa a redirecionar para a 443 (TLS 1.3/1.2). Crie um CNAME do seu domínio para o DNS do ALB, porque o certificado não cobre `*.elb.amazonaws.com`.

## Destruir

```bash
make destroy
```

O repositório ECR é esvaziado e removido junto com a stack (`EmptyOnDelete`).

## Custo estimado (us-east-1)

Cerca de **US$ 0,10/h (~US$ 2,50/dia)**. O NAT Gateway (~US$ 0,045/h, cobrado por hora cheia) é o maior item, seguido do ALB, de 2 tasks Fargate ARM 0.25 vCPU/0.5 GB e dos IPv4 públicos do NAT e do ALB. Confira na [AWS Pricing Calculator](https://calculator.aws/) antes de subir.

## Exceções aceitas do Checkov

`checkov -f infra/stack.yaml` → 35 aprovados, 7 pulados. Cada exceção está registrada no `Metadata` do recurso com justificativa:

| Check | Motivo |
|---|---|
| CKV_AWS_260, CKV_AWS_2, CKV_AWS_103 | ALB público na porta 80; vira redirect para HTTPS com `CERT_ARN` |
| CKV_AWS_91 | Access logs do ALB criariam um bucket sem uso numa demo de 1 dia |
| CKV_AWS_136, CKV_AWS_158 | KMS CMK no ECR e nos logs adiciona custo sem dado sensível |
| CKV_AWS_65 | Container Insights é cobrado por métrica; ALB e logs bastam |

Em produção, reavalie todas: subnets e NAT por AZ, interface endpoints, KMS, access logs, WAF, SBOM e assinatura de imagem com cosign.

# sing-box-rules Design

## Overview

Go программа, запускаемая по cron в GitHub Actions, которая регулярно запрашивает IP ranges у ipinfo API, компилирует sing-box rule-set (.srs) файлы и публикует их в ветку `rule-set`. Аналог SagerNet/sing-geoip, но с собственным источником данных.

## Repository Structure

```
sing-box-rules/
├── .github/
│   └── workflows/
│       └── update.yml              # ежедневный cron для обновления доменов
├── cmd/
│   └── sing-box-rules/
│       └── main.go                 # entry point
├── internal/
│   ├── config.go                   # загрузка и парсинг config/domains.json
│   ├── fetcher.go                  # HTTP запросы к ipinfo API
│   ├── transformer.go              # ipinfo JSON → sing-box rule-set JSON
│   ├── compiler.go                 # JSON → .srs через sing-box v1.13.7
│   ├── git.go                      # работа с git (ветка rule-set, commit, push)
│   └── scheduler.go                # определение устаревших доменов по git-истории
├── config/
│   └── domains.json                # конфиг доменов для обработки
├── custom-rules/                   # ручные JSON файлы в формате sing-box rule-set
└── go.mod
```

## Config Format (`config/domains.json`)

```json
[
  {
    "name": "github",
    "domain": "github.com",
    "interval_days": 30,
    "extra_domains": ["github.io", "ghcr.io", "github.githubassets.com", "githubusercontent.com"]
  },
  {
    "name": "amazon",
    "domain": "amazon.com",
    "interval_days": 7,
    "extra_domains": ["amazonaws.com", "cloudfront.net"]
  }
]
```

Fields:
- `name` — имя выходного .srs файла (e.g. `github.srs`)
- `domain` — домен для запроса к ipinfo API
- `interval_days` — минимальное время между обновлениями в днях
- `extra_domains` — дополнительные домены, добавляемые в `domain_suffix` без API-запроса

## ipinfo API

Endpoint: `GET https://ipinfo.io/widget/demo/{domain}?dataset=ranges`

Response format:
```json
{
  "domain": "github.com",
  "redirects_to": null,
  "num_ranges": 22,
  "ranges": ["2401:cf20::/32", "77.77.189.112/28", ...]
}
```

Rate limit: demo API имеет ограничение запросов в сутки. При достижении лимита (HTTP 429) или любой ошибке API — обработка прекращается, продолжение на следующем запуске.

## Data Transformation

ipinfo response → sing-box rule-set JSON:

```json
{
  "rules": [
    {
      "domain_suffix": ["github.com", "github.io", "ghcr.io", ...],
      "ip_cidr": ["2401:cf20::/32", "77.77.189.112/28", ...]
    }
  ],
  "version": 3
}
```

- `domain_suffix` — объединение основного домена и `extra_domains`
- `ip_cidr` — массив `ranges` из ipinfo ответа
- `version` — всегда 3

## SRS Compilation

JSON → .srs через sing-box v1.13.7 как библиотеку (`github.com/sagernet/sing-box/common/srs`).

Паттерн из sing-geoip:
1. Создать `option.PlainRuleSet` с rules
2. Вызвать `srs.Write(output, plainRuleSet)`

## Custom Rules

Директория `custom-rules/` содержит JSON файлы в формате sing-box rule-set (как в секции Data Transformation). Каждый JSON компилируется в .srs, имя файла сохраняется (e.g. `example.json` → `example.srs`).

Компиляция кастомных правил происходит при каждом запуске, но в git попадают только если содержимое изменилось.

## Git Strategy

### Ветка `rule-set`
- Содержит только `.srs` файлы
- Создаётся при первом запуске, если не существует
- Коммиты формата: `update: github, amazon (2 domains)` или `update: custom (3 rules)`
- При отсутствии изменений — пуш не происходит

### Auth
- `GITHUB_TOKEN` из GitHub Actions с правами `contents: write`

### Конфликты
- `git pull --rebase` перед пушем
- При конфликте — пропускаем, перезапуск на следующий день

## Main Flow

```
main()
  ├── LoadConfig("config/domains.json")
  ├── CheckoutRuleSetBranch()
  ├── DetermineOutdated(config) → отсортированный список доменов
  │     ├── git log --format=%cd --follow -- <name>.srs (в ветке rule-set)
  │     ├── фильтр: last_commit < now - interval_days
  │     └── файлы без коммитов = всегда устаревшие
  ├── ForEach outdated domain (greedy, priority по interval_days asc):
  │     ├── FetchRanges(domain) → ipinfo response
  │     │     └── при лимите/ошибке → break
  │     ├── Transform(ipinfo, extra_domains) → sing-box JSON
  │     ├── CompileToSRS(json) → .srs bytes
  │     └── Save(name + ".srs")
  ├── CompileCustomRules("custom-rules/") → .srs файлы
  ├── GitAddAll() + GitDiff()
  │     └── если изменений нет → exit 0
  └── GitCommit() + GitPush() в ветку rule-set
```

Important: all commits must be atomic — one logical change per commit.

Key points:
- При HTTP 429 или ошибке ipinfo — `break`, продолжение на следующий день
- Сортировка по `interval_days` (asc) — более частые обновления в приоритете
- Файлы без коммитов в `rule-set` считаются устаревшими всегда

## GitHub Actions Workflow

`update.yml`:
- **Trigger:** cron ежедневно (e.g. `0 8 * * *`) + manual dispatch
- **Steps:**
  1. Checkout с `fetch-depth: 0`
  2. Setup Go
  3. `go run ./cmd/sing-box-rules`
- **Permissions:** `contents: write` для пуша в `rule-set`

## Dependencies

- Go 1.22+
- `github.com/sagernet/sing-box` v1.13.7 (для `srs.Write`)

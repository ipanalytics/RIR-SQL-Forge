_English version: [README.md](README.md)_

# RIR-SQL-Forge

RIR-SQL-Forge — утилита на Go для построения локального справочника владельцев IP-адресов и abuse-контактов из публичных массовых данных RPSL. Она автоматически загружает базы RIPE, APNIC и AFRINIC, разбирает данные потоково, разрешает связи между организациями/контактами и публикует артефакты SQLite, DuckDB, Parquet и CSV, пригодные для операций безопасности, обработки жалоб (abuse), риск-систем и сетевой аналитики.


<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License"></a>
  <img src="https://img.shields.io/badge/status-active-success" alt="Project status">
  <img src="https://img.shields.io/badge/go-1.26+-00ADD8" alt="Go version">
  <img src="https://img.shields.io/badge/database-SQLite%20%7C%20DuckDB-003B57" alt="Databases">
  <img src="https://img.shields.io/badge/format-Parquet-4c1" alt="Parquet">
  <img src="https://img.shields.io/badge/dataset-RIPE%20%7C%20APNIC%20%7C%20AFRINIC-4c1" alt="Datasets">
  <img src="https://img.shields.io/badge/CI-GitHub%20Actions-2088FF" alt="CI">
  <img src="https://img.shields.io/badge/releases-db--YYYY--MM--DD-informational" alt="Release format">
</p>

> Документация: этот README. Автоматизированные наборы данных: GitHub Releases, формируемые `.github/workflows/build-db.yml`.

---

## Обзор

Массовые данные региональных интернет-регистраторов (RIR) полезны в эксплуатации, но исходный формат неудобен для выполнения запросов. Объекты сетей обычно не содержат прямых abuse-адресов; контакты нормализованы в отдельные объекты RPSL и связаны через handle-и (handles). RIR-SQL-Forge превращает эти сырые реестры в локальную базу данных, к которой можно обращаться с запросами без живых WHOIS-проверок.

Компилятор в настоящее время работает с публичными данными RPSL из:

| Реестр | Получение | Входные данные |
| --- | --- | --- |
| RIPE NCC | автоматическая загрузка | файлы разбиения `inetnum`, `inet6num`, `organisation`, `role` |
| APNIC | автоматическая загрузка | файлы разбиения `inetnum`, `inet6num`, `organisation`, `role`, `irt` |
| AFRINIC | автоматическая загрузка | объединённый файл `afrinic.db.gz` |
| LACNIC | локальный файл | `--lacnic-db /path/to/lacnic.db` |
| ARIN | флаг локального файла | `--arin-xml /path/to/arin.xml`; разбор XML намеренно не автоматизирован в этом MVP |

По умолчанию команда `rir-sql-forge compile` сама загружает публичные данные RIPE, APNIC и AFRINIC. Локальные/ручные источники являются дополнительными и пропускаются, если не указаны.

---

## Архитектура

```text
public RIR FTP/HTTPS
        |
        v
 streaming downloader  ---> temp source files
        |
        v
 RPSL stream parser     ---> networks / organisations / contacts
        |
        v
 SQLite bulk writer     ---> normalized tables
        |
        v
 flatten SQL            ---> SQLite / DuckDB / Parquet / CSV artifacts
```

Разрешение связей RPSL выполняется по ссылкам в реестре, а не путём текстового парсинга:

```text
network.org
  -> organisation.organisation
  -> organisation.abuse-c
  -> role/person.nic-hdl
  -> abuse-mailbox
```

Если `org` отсутствует, компилятор переходит к APNIC IRT или прямым эксплуатационным контактам объекта сети:

```text
network.mnt-irt
  -> irt
  -> abuse-mailbox

network.admin-c or network.tech-c
  -> role/person.nic-hdl
  -> abuse-mailbox or e-mail
```

Парсер читает абзацы RPSL из `io.Reader`, поддерживает вход в формате gzip, обрабатывает строки продолжения и передаёт типизированные записи в конвейер компилятора. Запись в SQLite использует подготовленные выражения (prepared statements) и транзакции с размером пакета по умолчанию 50 000 строк.

---

## Возможности

- Автоматическая загрузка публичных массовых данных RIPE, APNIC и AFRINIC.
- Потоковый парсер RPSL для обычного и gzip-входа.
- Нормализация `inetnum`, `inet6num`, `route` и `route6`.
- Реляционное разрешение контактов по объектам network, organisation, role, person и APNIC IRT.
- Вывод SQLite с использованием `modernc.org/sqlite` в качестве базовой линии совместимости.
- Вывод DuckDB для локальной аналитики и сценариев с интенсивным сканированием.
- Вывод Parquet для lakehouse, объектных хранилищ и конвейеров колоночной загрузки.
- Плоский экспорт CSV для систем-потребителей, которым не нужны типизированные колоночные данные.
- Опциональный локальный ввод RPSL от LACNIC.
- Явный флаг ARIN XML с безопасным пропуском.
- Еженедельная сборка GitHub Actions, публикующая сжатые артефакты баз данных.
- Тесты на основе фикстур для поведения парсера, путей отказа загрузчика, логики join, экспорта CSV и синтаксиса workflow.

---

## Быстрый старт

```bash
git clone https://github.com/ipanalytics/rir-sql-forge.git
cd rir-sql-forge

go test ./...
go build ./cmd/rir-sql-forge

./rir-sql-forge compile --output dist --work-dir tmp/rir-sql-forge
```

Команда compile автоматически загружает публичные наборы данных RIPE, APNIC и AFRINIC, если не задан флаг `--skip-download`.

Создаваемые файлы:

```text
dist/net_owner_directory.sqlite
dist/net_owner_directory.duckdb
dist/net_owner_directory.parquet
dist/net_owner_directory.csv
dist/net_owner_directory.stats.json
dist/net_owner_directory.stats.md
```

---

## Установка

### Из исходного кода

```bash
go install github.com/ipanalytics/rir-sql-forge/cmd/rir-sql-forge@latest
```

### Локальная сборка

```bash
go build -o rir-sql-forge ./cmd/rir-sql-forge
```

### Проверки при разработке

```bash
go test ./...
go test -race ./...
go vet ./...
```

---

## Использование

### Сборка из публичных реестров

```bash
rir-sql-forge compile \
  --output dist \
  --work-dir tmp/rir-sql-forge
```

Это обычный производственный путь. Компилятор загружает публичные исходные файлы RIPE, APNIC и AFRINIC, разбирает их и записывает результаты в форматах SQLite, DuckDB, Parquet и CSV.

### Использование локальной фикстуры или зеркала

```bash
rir-sql-forge compile \
  --skip-download \
  --source RIPE=internal/compiler/testdata/sample.db \
  --output dist
```

### Включение вручную полученных данных LACNIC

```bash
rir-sql-forge compile \
  --lacnic-db /data/rir/lacnic.db \
  --output dist
```

### Указание пути к XML-файлу ARIN

```bash
rir-sql-forge compile \
  --arin-xml /data/rir/arin.xml \
  --output dist
```

Текущая сборка записывает в журнал понятное сообщение о пропуске ARIN вместо попытки автоматизации ARIN с использованием учётных данных.

### Отключение публичного реестра

```bash
rir-sql-forge compile --afrinic=false
```

---

## Выходные данные и артефакты

| Артефакт | Формируется | Описание |
| --- | --- | --- |
| `net_owner_directory.sqlite` | локальная компиляция / CI | Нормализованная база данных SQLite плюс плоская таблица поиска |
| `net_owner_directory.duckdb` | локальная компиляция / CI | База данных DuckDB, содержащая `ip_owner_abuse_directory` |
| `net_owner_directory.parquet` | локальная компиляция / CI | Колончатый экспорт Parquet плоского каталога |
| `net_owner_directory.csv` | локальная компиляция / CI | Экспорт CSV из `ip_owner_abuse_directory` |
| `net_owner_directory.stats.json` | локальная компиляция / CI | Отчёт о насыщении набора данных в машиночитаемом формате |
| `net_owner_directory.stats.md` | локальная компиляция / CI | Сводка покрытия в формате Markdown, используемая в примечаниях к релизам |
| `net_owner_directory.sqlite.gz` | GitHub Actions | Сжатая база данных релиза |
| `net_owner_directory.duckdb.gz` | GitHub Actions | Сжатая база данных релиза DuckDB |
| `net_owner_directory.parquet` | GitHub Actions | Релизный артефакт Parquet |
| `net_owner_directory.csv.gz` | GitHub Actions | Сжатый релизный CSV |

GitHub Releases используют теги вида:

```text
db-YYYY-MM-DD
```

---

## Формат данных

База данных SQLite содержит нормализованные исходные таблицы и плоскую таблицу. DuckDB и Parquet содержат плоский рабочий набор данных.

Схема SQLite:

```sql
CREATE TABLE networks (
    cidr TEXT,
    rir TEXT,
    org_id TEXT,
    contact_id TEXT
);

CREATE TABLE organisations (
    org_id TEXT PRIMARY KEY,
    org_name TEXT,
    country TEXT,
    abuse_contact_id TEXT
);

CREATE TABLE contacts (
    contact_id TEXT PRIMARY KEY,
    abuse_email TEXT,
    tech_email TEXT
);
```

Затем компилятор строит плоскую рабочую таблицу:

```sql
CREATE TABLE ip_owner_abuse_directory AS
SELECT
    n.cidr,
    n.rir,
    o.org_name,
    o.country,
    COALESCE(NULLIF(c1.abuse_email, ''), NULLIF(c2.abuse_email, ''), NULLIF(c2.tech_email, '')) AS contact_email
FROM networks n
LEFT JOIN organisations o ON n.org_id = o.org_id
LEFT JOIN contacts c1 ON o.abuse_contact_id = c1.contact_id
LEFT JOIN contacts c2 ON n.contact_id = c2.contact_id;
```

Столбцы CSV:

| Столбец | Значение |
| --- | --- |
| `cidr` | Нормализованный префикс сети |
| `rir` | Метка исходного реестра |
| `org_name` | Название организации, если оно определено |
| `country` | Код страны организации, если он присутствует |
| `contact_email` | Предпочтителен ящик abuse; иначе резервный технический адрес |

<details>
<summary>Поля исходных объектов, используемые компилятором</summary>

| Объект | Поля |
| --- | --- |
| `inetnum`, `inet6num`, `route`, `route6` | `org`, `mnt-irt`, `admin-c`, `tech-c` |
| `organisation` | `organisation`, `org-name`, `country`, `abuse-c` |
| `role`, `person` | `nic-hdl`, `abuse-mailbox`, `e-mail` |
| `irt` | `irt`, `abuse-mailbox`, `e-mail` |

</details>

---

## Насыщение набора данных

При каждой компиляции рядом с артефактами данных записывается отчёт о насыщении:

```text
net_owner_directory.stats.json
net_owner_directory.stats.md
```

Отчёт также встраивается в примечания к релизам GitHub. Он должен ответить на практический вопрос, важный для этого набора данных: какая часть пространства реестров была сопоставлена с полезными данными о владении и контактах.

Отслеживаемые метрики:

| Метрика | Значение |
| --- | --- |
| `total_networks` | Строки в плоском каталоге |
| `networks_with_email` | Строки, в которых заполнено `contact_email` |
| `email_coverage_pct` | Насыщение контактными адресами по всем строкам сетей |
| `networks_with_org_name` | Строки, сопоставленные с названием организации |
| `org_name_coverage_pct` | Насыщение названиями организаций |
| `abuse_mailbox_matches` | Строки, сопоставленные через `organisation.abuse-c -> abuse-mailbox` |
| `fallback_tech_email_matches` | Строки, сопоставленные через резервные контакты `mnt-irt`, `admin-c` или `tech-c` |
| `contact_rows` | Разобранные строки контактов `role`, `person` и `irt` |

Низкое покрытие — это полезный сигнал, а не скрытая ошибка. Обычно это означает, что объекты вышестоящего реестра не раскрывают контакты, ссылка указывает на материалы ARIN/LACNIC, которые не были предоставлены, либо данные реестра неполны для данного диапазона.

### Последняя опубликованная статистика

Этот блок обновляется релизным workflow после успешной сборки набора данных.

<!-- DATASET_STATS_START -->
# Насыщенность набора данных

| Метрика | Значение |
| --- | ---: |
| Всего строк сетей | 7009199 |
| Строк с контактным email | 1771401 |
| Строк без контактного email | 5237798 |
| Покрытие email | 25.27% |
| Строк с названием организации | 562308 |
| Покрытие организаций | 8.02% |
| Строк со страной | 392359 |
| Покрытие стран | 5.60% |
| Совпадения по abuse-ящику | 447620 |
| Совпадения по резервным tech/admin email | 1323781 |
| Нормализованные строки сетей | 7009199 |
| Строки организаций | 118444 |
| Строки контактов | 216180 |

Низкое покрытие контактами обычно означает, что вышестоящие объекты RPSL не раскрывают abuse-ящик либо ссылка на контакт указывает на ограниченный/ручной источник данных реестра.
<!-- DATASET_STATS_END -->

---

## Примечания по эксплуатации

- Загрузка публичных реестров ограничена сетью и должна выполняться в CI или на хосте со стабильным исходящим доступом.
- Используйте `--work-dir` на постоянных раннерах для управления временным хранилищем исходных файлов.
- Компилятор удаляет и заново создаёт выходной файл SQLite при каждом запуске.
- Тесты не требуют доступа к реестру в реальном времени; они используют локальные фикстуры и имитацию HTTP-транспорта.
- Автоматизация релизов использует `GITHUB_TOKEN` и право доступа к репозиторию `contents: write`.
- Для воспроизводимых внутренних конвейеров зеркалируйте исходные файлы и запускайте с `--skip-download --source RIR=/path/to/file`.

---

## Область охвата проекта

RIR-SQL-Forge сосредоточен на преобразовании массовых данных реестров в локальные артефакты для запросов. Он предназначен для запланированных офлайн-сборок, а не для работы живого WHOIS-сервиса.

Входит в охват:

- приём публичных данных RPSL из RIPE/APNIC/AFRINIC
- локальные/ручные файлы-источники
- выходные форматы SQLite, DuckDB, Parquet и CSV
- публикация релизных артефактов через CI
- тесты корректности парсера и соединений (join)

Вне охвата текущей сборки:

- управление учётными данными для порталов реестров с ограниченным доступом
- размещаемые (hosted) API поиска
- WHOIS-запросы в реальном времени
- полная нормализация ARIN XML

---

## Варианты использования

- обогащение и маршрутизация данных для abuse-служб
- конвейеры оценки мошенничества и рисков
- обогащение телеметрии безопасности
- аналитика владения сетями
- офлайн-расследования, где live WHOIS-запросы медленны или ограничены по частоте
- запланированные внутренние наборы данных для команд SOC, CTI и инфраструктуры

---

## Ограничения

- Качество данных реестров различается в зависимости от источника и владельца объекта.
- Некоторые сети не сопоставляются с email-адресом после RPSL-соединений (join).
- Поддержка массового ARIN XML представлена локальным флагом, но в этой версии не разбирается.
- DuckDB и Parquet содержат плоское операционное представление; используйте SQLite, когда важно сохранение нормализованных исходных соединений.

---

## Структура каталогов

```text
.
├── cmd/rir-sql-forge/          # CLI entrypoint
├── internal/app/               # Command parsing and application wiring
├── internal/compiler/          # End-to-end compile orchestration
├── internal/artifacts/         # DuckDB and Parquet artifact writers
├── internal/db/                # SQLite schema, bulk inserts, flattening, CSV export
├── internal/downloader/        # Streaming HTTP downloads
├── internal/parser/            # RPSL stream parser and CIDR normalization
├── internal/sources/           # Public RIR source catalog
├── .github/workflows/          # Weekly release automation
└── site/                       # Repository visual assets
```

---

## Развёртывание

Репозиторий включает `.github/workflows/build-db.yml`.

Расписание:

```yaml
cron: '0 0 * * 0'
```

Workflow:

1. клонирует репозиторий
2. устанавливает Go
3. запускает тесты
4. собирает `rir-sql-forge`
5. загружает и компилирует публичные данные RIPE/APNIC/AFRINIC
6. записывает SQLite, DuckDB, Parquet, CSV и отчёты о насыщенности
7. сжимает выходные файлы SQLite, DuckDB и CSV
8. встраивает статистику насыщенности в примечания к релизу
9. создаёт или обновляет GitHub Release с тегом `db-YYYY-MM-DD`

Ручной запуск доступен через `workflow_dispatch`.

---

## Лицензия

RIR-SQL-Forge распространяется по лицензии [Apache License 2.0](./LICENSE).

---

## Отказ от ответственности

RIR-SQL-Forge обрабатывает данные реестров в том виде, в каком они опубликованы соответствующими RIR. Перед распространением производных наборов данных за пределами вашей организации ознакомьтесь с условиями каждого реестра.

# SEO Bot - Automated SEO Auditing System

A distributed microservices-based system for automated website crawling, parsing, technical SEO auditing, scoring, and report generation.

---

## 🏗️ Architecture Overview

The system consists of specialized Go microservices communicating via events and HTTP APIs:

- **API Gateway (`gateway`)**: Central entry point and routing for external requests.
- **Intake Service (`intake-service`)**: Handles audit job submission and validation.
- **Orchestrator Service (`orchestrator-service`)**: Coordinates audit workflows across microservices.
- **Crawler Service (`crawler-service`)**: Concurrently crawls target URLs and handles rate limits.
- **Parser Service (`parser-service`)**: Extracts DOM elements, meta tags, headers, links, and content.
- **On-Page Analyzer (`onpage-analyzer`)**: Evaluates on-page SEO metrics (title, description, headings, alt text).
- **Technical Auditor (`technical-auditor`)**: Inspects technical health (canonical tags, robots.txt, sitemaps, status codes).
- **Keyword Extractor (`keyword-extractor`)**: Computes keyword frequencies, density, and relevance.
- **Scoring Service (`scoring-service`)**: Computes weighted SEO scores and benchmarks.
- **Report Service (`report-service`)**: Aggregates analysis into detailed, structured audit reports.
- **Notification Service (`notification-service`)**: Sends notifications and alerts upon audit completion.
- **Database (`postgres`)**: PostgreSQL schema and migration scripts for persistent audit history.

---

## 🚀 Getting Started

### Prerequisites
- [Docker](https://www.docker.com/) & Docker Compose
- [Go (1.21+)](https://golang.org/) (for local development)

### Quick Start with Docker Compose

1. Clone the repository:
   ```bash
   git clone <REPO_URL>
   cd seo-bot
   ```

2. Configure environment variables:
   ```bash
   cp backend/.env.example backend/.env
   ```

3. Launch all services:
   ```bash
   cd backend
   docker compose up -d --build
   ```

4. Verify services:
   ```bash
   docker compose ps
   ```

---

## 📁 Repository Structure

```text
seo-bot/
├── backend/
│   ├── crawler-service/
│   ├── gateway/
│   ├── intake-service/
│   ├── keyword-extractor/
│   ├── notification-service/
│   ├── onpage-analyzer/
│   ├── orchestrator-service/
│   ├── parser-service/
│   ├── postgres/
│   ├── report-service/
│   ├── scoring-service/
│   ├── technical-auditor/
│   ├── docker-compose.yml
│   └── .env.example
├── frontend/
├── .gitignore
└── README.md
```

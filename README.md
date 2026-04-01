# ANAM-AI Patient History EMR Backend

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://go.dev)
[![GORM](https://img.shields.io/badge/GORM-1.30+-orange.svg)](https://gorm.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-blue.svg)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-Proprietary-red.svg)](LICENSE)

**A high-performance Go backend for managing Electronic Medical Records (EMR) and syncing clinical notes from the ANAM-AI history-taking agent.**

</div>

## 🎯 Overview

The **ANAM-AI EMR Backend** acts as the crucial data management tier for the AI-powered patient history-taking application. Since the AI Agent processes complex Roman Urdu natural language and extracts structured summaries, this backend system handles the storage, relational mapping, and synchronization of these validated medical records.

It addresses the fundamental flaws of legacy systems in the Pakistani market by integrating an "Integration Bridge" approach—acting not as an isolated island but as a structured data sink tailored for both Junior Doctors and Private Hospital Administrations.

### 🚀 Key Features

- **🛡️ Secure Data Access** - Manages user authentication and prevents unauthorized EMR access.
- **🔄 Intermittent Connectivity Sync** - Anticipates inconsistent internet in Pakistani hospitals by providing robust API endpoints that support offline-sync workflows.
- **✅ High-throughput Operations** - Engineered in Go to smoothly handle the heavy concurrency required in high-volume public and private OPDs (60-100 patients per MO shift).
- **📝 Automatic Migration** - Employs GORM to automatically generate structured tables and enforce relational integrity directly to PostgreSQL.

### 🏗️ System Architecture

- **`main.go`**: Entry point orchestrating server setup, routing, and database connection.
- **`database/`**: Sets up the PostgreSQL connection using GORM and manages migrations.
- **`models/`**: Defines the rigorous GORM structs modeling Patients, Doctors, and generated consultation notes.

## 🛠️ Technology Stack

### Core Technologies
- **Go 1.24.4** - Highly concurrent, performant primary language
- **GORM (v1.31.x)** - Developer-friendly Object Relational Mapper for Go
- **PostgreSQL** - Scalable, enterprise-ready relational database
- **lib/pq** - Pure Go Postgres driver

## 📁 Repository Structure

```
.
├─ database/            # Database initialization and AutoMigrate logic
├─ models/              # GORM structural definitions for Users, Patients, and Logs
├─ go.mod               # Go module dependencies
├─ go.sum               # Dependency checksums
└─ main.go              # Application entry point
```

## 🚀 Quick Start

### Prerequisites

- **Go 1.24+**
- **Docker** (optional, but recommended for local DB spin-up)
- **PostgreSQL 15+**

### Installation

1. **Clone the repository and enter the directory**
   ```bash
   cd ThatOneMvp-Backend
   ```

2. **Download Dependencies**
   ```bash
   go mod download
   ```

3. **Database Configuration**
   Ensure your PostgreSQL instance is running. Provide the DSN (Data Source Name) to the `ConnectDB` via environment variables or straight in the `database` logic if overriding defaults.

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=anam_db
```

### Running the Application

Start the backend service:

```bash
go run main.go
```
The server will initialize the connection and apply any pending GORM auto-migrations before accepting incoming connections.

## 📝 Integration Notes

This backend heavily interacts with the `ThatOneMvp-Agent`.
To process AI-generated files safely:
1. Audio is sent to the Agent for TTS & structural LLM extraction.
2. The Agent requests the doctor to Verify the transcript.
3. The Agent securely posts the verified JSON data to this Go Backend.

## 👥 Authors

- **[22i-0767, 22i-0911, 22i-0891, 22i-0928]**

# ⏱️ Ionic TimeTracker

A full-stack **Time Tracking Application** built with:

- ⚡ **Frontend**: [Ionic](https://ionicframework.com/) + Angular + Capacitor  
- 🔙 **Backend**: [Golang](https://go.dev/) (REST API with JWT auth)  
- 🐘 **Database**: PostgreSQL (via Docker)  
- 🐳 **Containerization**: Docker & docker-compose  
- 🚀 **CI/CD**: GitHub Actions  

This project allows users to **register, login, start/stop time sessions, and view their daily totals** – accessible both via the **web app** and the **iOS app**.

---

## 📂 Project Structure

```
ionic-timetraker/
│── backend/          # Golang backend API (REST, JWT, PostgreSQL)
│── frontend/         # Ionic + Angular + Capacitor mobile/web app
│── db/               # Database migrations / init scripts
│── docker-compose.yml# Docker setup (db + backend + pgAdmin)
│── .github/workflows # GitHub Actions (CI/CD pipelines)
```

---

## 🛠️ Installation & Setup

### 1. Clone the Repository
```bash
git clone https://github.com/Abudi7/ionic-timetraker.git
cd ionic-timetraker
```

### 2. Start with Docker
```bash
docker-compose up --build
```

This starts:
- `db` → PostgreSQL 16  
- `api` → Go backend (exposed on `http://localhost:8087`)  
- `frontend` → Ionic/Angular app (dev mode on `http://localhost:8100`)  
- `pgadmin` (optional) → GUI for DB on `http://localhost:8090`  

### 3. Frontend Development (Ionic)
```bash
cd frontend
npm install
ionic serve
```

Runs at: 👉 `http://localhost:8100`

### 4. iOS App (Capacitor)
```bash
cd frontend
npx cap sync ios
npx cap open ios
```

Then run on iPhone simulator via **Xcode**.

---

## 🔑 Environment Variables

Backend requires:
```env
PORT=8080
DATABASE_URL=postgres://app:apppass@db:5432/timetrac?sslmode=disable
CORS_ORIGIN=http://localhost:8100
JWT_SECRET=supersecret_change_me
TZ=Europe/Vienna
```

Frontend uses:
```ts
// src/environments/environment.ts
export const environment = {
  production: false,
  apiBase: 'http://localhost:8087'
};
```

---

## ✅ Features

- 🔐 User Registration & JWT Login  
- ⏯️ Start / Stop time tracking  
- 📊 View today’s sessions & totals  
- 📱 Responsive Web + iOS app (Capacitor)  
- 🐳 Full Dockerized stack  
- ⚡ CI/CD with GitHub Actions  

---

## 🤖 CI/CD (GitHub Actions)

Workflow: `.github/workflows/ci.yml`  
- Lints & builds frontend  
- Runs Go backend tests  
- Optionally builds Docker images  

---

## 🖥️ Screenshots

| Web (Browser) | iOS (Simulator) |
|---------------|-----------------|
| <img width="1678" height="793" alt="Web App" src="https://github.com/user-attachments/assets/6e6ac81c-1904-418a-af88-fb30c4de1e7b" /> | <img width="391" height="867" alt="iOS App" src="https://github.com/user-attachments/assets/d7df4e57-28b2-4e6c-b224-b6bc5b6c9fac" /> |

---

## 👨‍💻 Author

**Abdulrhman Alshalal**  
🌍 Graz, Austria  
📧 casper.king14@gmail.com  
🔗 [GitHub Profile](https://github.com/Abudi7)

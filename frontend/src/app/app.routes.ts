// src/app/app.routes.ts
import { Routes } from '@angular/router';
import { AuthGuard } from './../core/auth.guard';

export const routes: Routes = [
  { path: '', redirectTo: 'home', pathMatch: 'full' }, // ← افتح Home إذا فيه توكن
  { path: 'login', loadComponent: () => import('../pages/login/login.page').then(m => m.LoginPage) },
  { path: 'home',  canActivate: [AuthGuard], loadComponent: () => import('../pages/home/home.page').then(m => m.HomePage) },
  { path: '**', redirectTo: 'home' }
];

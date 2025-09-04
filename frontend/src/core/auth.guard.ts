// src/app/core/auth.guard.ts

import { Injectable } from '@angular/core';
import { CanActivate, Router, UrlTree } from '@angular/router';
import { AuthService } from './auth.service';

// AuthGuard is used to protect routes that require the user to be logged in.
@Injectable({ providedIn: 'root' })
export class AuthGuard implements CanActivate {
  constructor(private auth: AuthService, private router: Router) {}

  // canActivate is called before the route is activated.
  // If a valid token exists → allow access (true).
  // If not logged in → redirect the user to '/login'.
  canActivate(): boolean | UrlTree {
    return this.auth.token ? true : this.router.createUrlTree(['/login']);
  }
}

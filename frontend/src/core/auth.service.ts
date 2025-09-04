// src/app/core/auth.service.ts
import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, tap } from 'rxjs';
import { environment } from '../environments/environment';

export type User = { id: number; email: string; name?: string; avatarUrl?: string };

type LoginRes = { token: string; user: User; exp: string };

@Injectable({ providedIn: 'root' })
export class AuthService {
  private base = environment.apiBase;
  private tokenKey = 'tt_token';
  private userKey  = 'tt_user';

  private _isLoggedIn$ = new BehaviorSubject<boolean>(!!localStorage.getItem(this.tokenKey));
  isLoggedIn$ = this._isLoggedIn$.asObservable();

  private _user$ = new BehaviorSubject<User | null>(
    localStorage.getItem(this.userKey) ? JSON.parse(localStorage.getItem(this.userKey)!) as User : null
  );
  user$ = this._user$.asObservable();

  constructor(private http: HttpClient) {}

  get token(): string | null { return localStorage.getItem(this.tokenKey); }
  get currentUser(): User | null { return this._user$.value; }

  /** Call on app start to restore/refresh profile */
  bootstrap() {
    if (!this.token) { this._user$.next(null); return; }
    // hit your backend to be 100% in sync with DB
    this.me().subscribe({ next: (u) => this.setUser(u), error: () => this.clear() });
  }

  /** Login: store token + user (from backend) */
  login(email: string, password: string) {
    return this.http.post<LoginRes>(`${this.base}/auth/login`, { email, password })
      .pipe(tap(res => {
        localStorage.setItem(this.tokenKey, res.token);
        this._isLoggedIn$.next(true);
        this.setUser(res.user);               // keep user from DB
      }));
  }

  register(email: string, password: string) {
    return this.http.post(`${this.base}/auth/register`, { email, password });
  }

  /** Logout: clear everything */
  logout() {
    return this.http.post(`${this.base}/auth/logout`, {}).pipe(tap(() => this.clear()));
  }

  /** GET the fresh user from DB */
  me() {
    return this.http.get<User>(`${this.base}/auth/me`);
  }

  /** Helpers */
  private setUser(u: User) {
    this._user$.next(u);
    localStorage.setItem(this.userKey, JSON.stringify(u));
  }

  clear() {
    localStorage.removeItem(this.tokenKey);
    localStorage.removeItem(this.userKey);
    this._user$.next(null);
    this._isLoggedIn$.next(false);
  }
}

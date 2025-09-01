import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, tap } from 'rxjs';
import { environment } from '../environments/environment';

type LoginRes = { token: string; user: { id: number; email: string }; exp: string };

@Injectable({ providedIn: 'root' })
export class AuthService {
  private base = environment.apiBase;
  private key = 'tt_token';
  private _isLoggedIn$ = new BehaviorSubject<boolean>(!!localStorage.getItem(this.key));
  isLoggedIn$ = this._isLoggedIn$.asObservable();

  constructor(private http: HttpClient) {}

  get token(): string | null { return localStorage.getItem(this.key); }

  login(email: string, password: string) {
    return this.http.post<LoginRes>(`${this.base}/auth/login`, { email, password })
      .pipe(tap(res => {
        localStorage.setItem(this.key, res.token);
        this._isLoggedIn$.next(true);
      }));
  }

  register(email: string, password: string) {
    return this.http.post(`${this.base}/auth/register`, { email, password });
  }

  logout() {
    return this.http.post(`${this.base}/auth/logout`, {}).pipe(tap(() => this.clear()));
  }

  clear() {
    localStorage.removeItem(this.key);
    this._isLoggedIn$.next(false);
  }
}

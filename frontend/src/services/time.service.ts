import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { environment } from '../environments/environment';

@Injectable({ providedIn: 'root' })
export class TimeService {
  private base = environment.apiBase;
  constructor(private http: HttpClient) {}
  start()        { return this.http.post(`${this.base}/api/time/start`, {}); }
  stop()         { return this.http.post(`${this.base}/api/time/stop`, {}); }
  sessionsToday(){ return this.http.get<any[]>(`${this.base}/api/time/sessions`); }
  totalToday()   { return this.http.get<{ totalMinutes: number }>(`${this.base}/api/time/total-today`); }
}

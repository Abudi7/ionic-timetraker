import { Component, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  IonicModule, ToastController
} from '@ionic/angular';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';

import { TimeService } from '../../services/time.service';
import { AuthService } from '../../core/auth.service';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [CommonModule, IonicModule, FormsModule],
  templateUrl: './home.page.html',
  styleUrls: ['./home.page.scss']
})
export class HomePage implements OnInit, OnDestroy {
  private time = inject(TimeService);
  private toast = inject(ToastController);
  private auth = inject(AuthService);
  private router = inject(Router);

  loading = false;

  // API state
  total = 0;                 // finished minutes today
  sessions: any[] = [];

  // live state
  isRunning = false;
  runningStart?: Date;
  liveSeconds = 0;
  private tick?: any;

  // UI helpers
  skeletons = Array.from({ length: 3 });
  userName?: string;
  userAvatar?: string;

  // Derived
  get liveMinutes(): number { return Math.floor(this.liveSeconds / 60); }
  get displayedTotal(): number { return this.total + (this.isRunning ? this.liveMinutes : 0); }
  get liveDisplay(): string { return this.formatMMSS(this.liveSeconds); }

  async ngOnInit() {
    // Optional: read user data if your AuthService exposes it (e.g. an observable `user$`
    // or a getter like `getCurrentUser()`); keep fallbacks so the template stays safe.
    try {
      // Example alternatives you can enable if your AuthService provides them:
      // const u = await this.auth.user$?.toPromise?.();
      // const u = this.auth.getCurrentUser?.();
      // For now use a safe default so compilation succeeds without relying on a specific API:
      this.userName = 'User';
      this.userAvatar = undefined;
    } catch {}
    this.refresh();
  }

  ngOnDestroy() { this.clearTicker(); }

  async doRefresh(ev: CustomEvent) {
    await this.refresh();
    (ev.target as HTMLIonRefresherElement).complete();
  }

  async refresh() {
    this.loading = true;
    try {
      const t = await this.time.totalToday().toPromise();
      this.total = t?.totalMinutes ?? 0;

      this.sessions = await this.time.sessionsToday().toPromise() ?? [];
      const open = this.sessions.find(s => !s.endTime);
      this.isRunning = !!open;

      if (open) {
        this.runningStart = new Date(open.startTime);
        this.startTicker();
      } else {
        this.runningStart = undefined;
        this.liveSeconds = 0;
        this.clearTicker();
      }
    } finally {
      this.loading = false;
    }
  }

  // Unified toggle for primary CTA and FAB
  toggle() {
    if (this.isRunning) { this.stop(); } else { this.start(); }
  }

  start() {
    if (this.isRunning || this.loading) return;
    this.loading = true;
    this.time.start().subscribe({
      next: async () => {
        (await this.toast.create({ message: 'Started ✅', duration: 900, position: 'top' })).present();
        this.refresh();
      },
      error: async e => (await this.toast.create({
        message: e?.error || 'Already running?',
        color: 'warning', duration: 1400, position: 'top'
      })).present(),
      complete: () => (this.loading = false),
    });
  }

  stop() {
    if (!this.isRunning || this.loading) return;
    this.loading = true;
    this.time.stop().subscribe({
      next: async () => {
        (await this.toast.create({ message: 'Stopped ✅', duration: 900, position: 'top' })).present();
        this.refresh();
      },
      error: async e => (await this.toast.create({
        message: e?.error || 'No open session',
        color: 'warning', duration: 1400, position: 'top'
      })).present(),
      complete: () => (this.loading = false),
    });
  }

  logout() {
    this.auth.logout().subscribe({
      complete: () => this.router.navigateByUrl('/login', { replaceUrl: true }),
      error:    () => this.router.navigateByUrl('/login', { replaceUrl: true })
    });
  }

  viewAllDays() {
    // TODO: implement navigation when you add history page
    // this.router.navigateByUrl('/history');
  }

  // --- live ticker ---
  private startTicker() {
    this.clearTicker();
    this.updateLive();
    this.tick = setInterval(() => this.updateLive(), 1000);
  }
  private clearTicker() {
    if (this.tick) { clearInterval(this.tick); this.tick = undefined; }
  }
  private updateLive() {
    if (!this.runningStart) { this.liveSeconds = 0; return; }
    const now = Date.now();
    const start = this.runningStart.getTime();
    const diff = Math.max(0, Math.floor((now - start) / 1000));
    this.liveSeconds = diff;
  }
  private formatMMSS(totalSeconds: number): string {
    const mm = Math.floor(totalSeconds / 60);
    const ss = totalSeconds % 60;
    const pad = (n: number) => n.toString().padStart(2, '0');
    return `${pad(mm)}:${pad(ss)}`;
  }
}

// src/main.ts
import { bootstrapApplication } from '@angular/platform-browser';
import { importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideIonicAngular } from '@ionic/angular/standalone';
import { HttpClientModule, HTTP_INTERCEPTORS } from '@angular/common/http';

import { AppComponent } from './app/app.component';
import { routes } from './app/app.routes';
import { AuthInterceptor } from './core/auth.interceptor';
import { AuthService } from './core/auth.service';

import { Capacitor } from '@capacitor/core';
import { StatusBar, Style } from '@capacitor/status-bar';

/** Configure native status bar to match our translucent toolbar (iOS/Web parity). */
async function setupStatusBar() {
  if (!Capacitor.isNativePlatform()) return;
  try {
    await StatusBar.setOverlaysWebView({ overlay: true });
    await StatusBar.setStyle({ style: Style.Light }); // switch to Style.Dark if your header is light
  } catch {
    // Non-fatal: status bar config not supported on this platform
  }
}

/** Bootstrap Angular + Ionic, then hydrate auth state from backend (/auth/me). */
bootstrapApplication(AppComponent, {
  providers: [
    // Force a single visual language across Web + iOS. Use { mode: 'ios' } if you prefer that look.
    provideIonicAngular({ mode: 'md' }),

    // App routes
    provideRouter(routes),

    // HttpClient + our auth header injector
    importProvidersFrom(HttpClientModule),
    { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true },
  ],
})
  .then(async appRef => {
    // Restore current user from DB if a token exists (keeps UI in sync after reloads).
    const auth = appRef.injector.get(AuthService);
    auth.bootstrap();

    // Align native chrome with our UI
    await setupStatusBar();
  })
  .catch(err => {
    // Surface bootstrap errors early in dev
    console.error('[Bootstrap] Failed to start app:', err);
  });

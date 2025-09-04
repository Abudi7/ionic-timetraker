import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { IonicModule, ToastController } from '@ionic/angular';
import { Router } from '@angular/router';
import { AuthService } from '../../core/auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule, IonicModule],
  templateUrl: './login.page.html',
  styleUrls: ['./login.page.scss'],
})
export class LoginPage {
  email = 'test1@demo.io';
  password = 'secret123';
  showPass = false;
  loading = false;
  year = new Date().getFullYear();

  constructor(
    private auth: AuthService,
    private router: Router,
    private toast: ToastController
  ) {}

  doLogin() {
    if (this.loading) return;
    this.loading = true;
    this.auth.login(this.email, this.password).subscribe({
      next: async () => {
        (await this.toast.create({ message: 'Logged in ✅', duration: 1000, position: 'top' })).present();
        this.router.navigateByUrl('/home', { replaceUrl: true });
      },
      error: async (e: any) => {
        const msg = e?.error?.message || e?.error || 'Login failed';
        (await this.toast.create({ message: msg, color: 'danger', duration: 1500, position: 'top' })).present();
      },
      complete: () => (this.loading = false),
    });
  }

  quickRegister() {
    if (this.loading) return;
    this.auth.register(this.email, this.password).subscribe({
      next: async () => (await this.toast.create({ message: 'Registered ✅', duration: 1000, position: 'top' })).present(),
      error: async (e: any) => {
        const msg = e?.error?.message || e?.error || 'Register failed';
        (await this.toast.create({ message: msg, color: 'danger', duration: 1500, position: 'top' })).present();
      }
    });
  }
}

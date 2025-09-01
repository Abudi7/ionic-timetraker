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
  loading = false;

  constructor(
    private auth: AuthService,
    private router: Router,
    private toast: ToastController
  ) {}

  doLogin() {
    this.loading = true;
    this.auth.login(this.email, this.password).subscribe({
      next: () => {
        this.toast.create({ message: 'Logged in ✅', duration: 1200 })
          .then(t => t.present());
        this.router.navigateByUrl('/home', { replaceUrl: true });
      },
      error: (e: any) => {
        const msg = e?.error?.message || e?.error || 'Login failed';
        this.toast.create({ message: msg, color: 'danger', duration: 1600 })
          .then(t => t.present());
      },
      complete: () => { this.loading = false; }
    });
  }

  quickRegister() {
    this.auth.register(this.email, this.password).subscribe({
      next: () => {
        this.toast.create({ message: 'Registered ✅', duration: 1200 })
          .then(t => t.present());
      },
      error: (e: any) => {
        const msg = e?.error?.message || e?.error || 'Register failed';
        this.toast.create({ message: msg, color: 'danger', duration: 1600 })
          .then(t => t.present());
      }
    });
  }
}

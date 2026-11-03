class CastAmbiguity {
    void m() {
        x = (a) + b; x = (a) + b + c; x = (a) + b * c; x = (a) + b.c; x = (a) + b[0]; x = (a) + m(); x = (a) + 1;
        x = (a) - b * c; x = (a) + b * c + d; x = (a) + b / c; x = (a) + b << c; x = (a) + b > c; x = (a) + b++;
        x = (a) + -b; x = (a) - b - c; x = (a) * b; f((a) + b * c);
        x = k * (a) + 0.1 * m(1); y = k * (a) + b; z = (a) + b * c; w = k + (a) - b;
        x = k + (a) + b * c; x = k - (a) - b % c; f((a) - b * c, (a) - b); if ((a) - b * c > 0) {}
        x = (a) + +b * c; x = (a) - b * c * d; x = (a.b) - c * d; x = k * (a) - b; x = (a) - b.c * d;
        x = ((a) - b * c); x = (a) - (b) * c; x = (a) + 1 * 2; x = (a) - b instanceof C;
        x = k < (a) + b * c; x = -(a) + b * c; x = k * (a) + b * c;
        a = (char) i++; b = (Foo) i++; c = (char) -i; d = (int) x--; e = (Foo) x.y++; f = (char) ++i;
        vc[F] = 0.2 * kpow4 * (L_Ax5) + 0.1 * Math.pow(1.0 - kpow4, 2.0) * Math.pow(L_Ax5, 1.0 / 3.0);
    }
}

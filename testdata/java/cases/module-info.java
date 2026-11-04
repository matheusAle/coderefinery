@Deprecated
open module demo.mod {
    requires transitive static java.base;
    requires java.sql;
    exports demo.api;
    exports demo.internal to demo.a, demo.b;
    opens demo.res to demo.c;
    uses demo.spi.Service;
    provides demo.spi.Service with demo.impl.A, demo.impl.B;
    provides X with Y;
}

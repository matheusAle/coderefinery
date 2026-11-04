// leading comment
/* block */ package /* inside */ demo.c; // trailing
import java.util.List; /* after import */

/**
 * Javadoc.
 */
@Deprecated // why
public class Comments /* name */ extends /* sup */ Object {
    // first member
    int x = /* value */ 1; // after field

    /* before method */
    @Override /* between */ public String toString() /* before body */ {
        // only a comment
        return "c" /* tail */ + x;
    } // after method

    void m(int a /* p */, /* q */ int b) {
        if (a > b) // then
            a++;
        else /* else */ b++;
        switch (a) {
            // in switch
            case 1: // label
                break;
            /* before default */
            default:
                // end
        }
        int[] arr = { /* empty */ };
        foo(/* nothing */);
        List</* T */ String> l = null;
    }
    // last member
}
// eof comment

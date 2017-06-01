import java.io.*;

public class Func {
    public static void main(String[] args) throws IOException {
        BufferedReader bufferedReader = new BufferedReader(new InputStreamReader(System.in));

        String name = bufferedReader.readLine();
        name = (name == null) ? "world"  : name;

        System.out.println("Hello, " + name + "!");
    }

}

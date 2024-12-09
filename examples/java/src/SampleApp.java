import com.sun.net.httpserver.HttpServer;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpExchange;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.IOException;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.URL;
import java.time.format.DateTimeFormatter;
import java.time.ZonedDateTime;


public class SampleApp {

    public static void main(String[] args) {
        try {
            String hostname = InetAddress.getLocalHost().getHostName();
            String version = System.getProperty("java.runtime.version");
            String remoteUrl = "https://whoami.local.test";

            HttpServer server = HttpServer.create(new InetSocketAddress(8080), 0);

            DateTimeFormatter formatter = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ssZZ");

            server.createContext("/", new HttpHandler() {
                @Override
                public void handle(HttpExchange exchange) throws IOException {
                    String iso8601 = ZonedDateTime.now().format(formatter);

                    String response = "Hi, I'm Java/" + version + " service running on '" + hostname + "' host.\n\n";
                    response += "Time is " + iso8601 + "\n\n";
                    response += "Rendering " + remoteUrl + " page\n";

                    for (int i = 0; i <= 2; i++) {
                        response += "\nrequest (" + (i+1) + "):\n\n";

                        try {
                            response += fetchRemotePage(remoteUrl);
                        } catch (Exception e) {
                            response += indentLine("# Error: " + e.getMessage()+ "\n");
                        }
                    }

                    response += "\nThank you!\n";

                    exchange.sendResponseHeaders(200, response.getBytes().length);
                    OutputStream os = exchange.getResponseBody();
                    os.write(response.getBytes());
                    os.close();
                }
            });

            server.setExecutor(null);
            System.out.println("Server is running on http://localhost:8080/");
            server.start();
        } catch (IOException e) {
            e.printStackTrace();
        }
    }

    private static String fetchRemotePage(String remoteUrl) throws IOException {
        URL url = new URL(remoteUrl);
        HttpURLConnection con = (HttpURLConnection) url.openConnection();

        con.setRequestMethod("GET");

        int responseCode = con.getResponseCode();
        String response = "";

        if (responseCode == HttpURLConnection.HTTP_OK) {
            BufferedReader in = new BufferedReader(new InputStreamReader(con.getInputStream()));
            String line;
            while ((line = in.readLine()) != null) {
                if (expectedLine(line)) {
                    response += indentLine(line.trim() + "\n");
                }
            }
            in.close();
        } else {
            throw new IOException("responded with code " + responseCode);
        }

        return response;
    }

    private static String indentLine(String line) {
        return "    " + line;
    }

    private static boolean expectedLine(String line) {
        if (line.contains("Hostname")) {
            return true;
        }

        if (line.contains("IP") && line.contains(".") && !line.contains("127.0.0.")) {
            return true;
        }

        return false;
    }
}

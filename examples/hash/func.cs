using System;
using System.Text;
using System.Security.Cryptography;
using System.IO;

namespace ConsoleApplication
{
    public class Program
    {
        public static void Main(string[] args)
        {
            // if nothing is being piped in, then exit
            if (!IsPipedInput())
                return;

            var input = Console.In.ReadToEnd();
            var stream = DownloadRemoteImageFile(input);
            var hash = CreateChecksum(stream);
            Console.WriteLine(hash);
        }

        private static bool IsPipedInput()
        {
            try
            {
                bool isKey = Console.KeyAvailable;
                return false;
            }
            catch
            {
                return true;
            }
        }
        private static byte[] DownloadRemoteImageFile(string uri)
        {

            var request = System.Net.WebRequest.CreateHttp(uri);
            var response = request.GetResponseAsync().Result;
            var stream = response.GetResponseStream();
            using (MemoryStream ms = new MemoryStream())
            {
                stream.CopyTo(ms);
                return ms.ToArray();
            }
        }
        private static string CreateChecksum(byte[] stream)
        {
            using (var md5 = MD5.Create())
            {
                var hash = md5.ComputeHash(stream);
                var sBuilder = new StringBuilder();

                // Loop through each byte of the hashed data
                // and format each one as a hexadecimal string.
                for (int i = 0; i < hash.Length; i++)
                {
                    sBuilder.Append(hash[i].ToString("x2"));
                }

                // Return the hexadecimal string.
                return sBuilder.ToString();
            }
        }
    }
}

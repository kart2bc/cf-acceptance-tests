using System;
using System.Web;
using System.Web.Http;
using System.IO;
using System.Text;

namespace Nora
{
    public class WebApiApplication : HttpApplication
    {
        protected void Application_Start()
        {
                var autoFlushError = new StreamWriter(Console.OpenStandardError(), Console.OutputEncoding)
    {
        AutoFlush = true
    };
    Console.SetError(autoFlushError);
            GlobalConfiguration.Configure(WebApiConfig.Register);
        }

        void Application_Error(Object sender, EventArgs e)
        {
            var exception = Server.GetLastError();
            if (exception == null)
                return;

            Console.WriteLine("error: " + exception);
        }
    }
}
package com.fnproject.fn.examples;

import com.fnproject.fn.api.FnConfiguration;
import com.fnproject.fn.api.RuntimeContext;
import com.fnproject.springframework.function.SpringCloudFunctionInvoker;
import org.springframework.cloud.function.context.ContextFunctionCatalogAutoConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;
import reactor.core.publisher.Flux;

import java.util.Arrays;
import java.util.Collections;
import java.util.Date;
import java.util.List;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.function.Supplier;

@Configuration
@Import(ContextFunctionCatalogAutoConfiguration.class)
public class SCFExample {
    @FnConfiguration
    public static void configure(RuntimeContext ctx) {
        ctx.setInvoker(new SpringCloudFunctionInvoker(SCFExample.class));
    }

    // Unused - see https://github.com/fnproject/fdk-java/issues/113
    public void handleRequest() { }

    public static class B{
        private String xxx;
        public B() {}

        public B(String xxx) {
            this.xxx = xxx;
        }

        public String getXxx() {
            return xxx;
        }

        public void setXxx(String xxx) {
            this.xxx = xxx;
        }
    }

    @Bean
    public Function<B, String> consumer(){
        return B::getXxx;
    }

}

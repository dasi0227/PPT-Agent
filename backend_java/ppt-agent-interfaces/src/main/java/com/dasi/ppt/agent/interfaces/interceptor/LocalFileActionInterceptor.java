package com.dasi.ppt.agent.interfaces.interceptor;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.jspecify.annotations.NonNull;
import org.springframework.web.cors.CorsUtils;
import org.springframework.web.servlet.HandlerInterceptor;

import java.net.InetAddress;
import java.net.UnknownHostException;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

public class LocalFileActionInterceptor implements HandlerInterceptor {

    @Override
    public boolean preHandle(HttpServletRequest request, @NonNull HttpServletResponse response, @NonNull Object handler) {
        if (notLoopback(request.getRemoteAddr()) || notLoopback(request.getServerName())) {
            throw invalid("请从本机应用页面操作文件");
        }

        String site = request.getHeader("Sec-Fetch-Site");
        if (site != null && !site.equals("same-origin") && !site.equals("none")) {
            throw invalid("请从本机应用页面操作文件");
        }
        try {
            if (CorsUtils.isCorsRequest(request)) {
                throw invalid("请从本机应用页面操作文件");
            }
        } catch (IllegalArgumentException e) {
            throw invalid("请求来源无效");
        }
        return true;
    }

    private static boolean notLoopback(String host) {
        if ("localhost".equalsIgnoreCase(host)) {
            return false;
        }
        if (host == null || !(host.matches("127(?:\\.[0-9]{1,3}){3}") || host.contains(":"))) {
            return true;
        }
        try {
            return !InetAddress.getByName(host).isLoopbackAddress();
        } catch (UnknownHostException e) {
            return true;
        }
    }
}

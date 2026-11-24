package com.dasi.ppt.agent.interfaces.handler;

import com.dasi.ppt.agent.domain.shared.exception.BusinessException;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.dasi.ppt.agent.interfaces.controller.asset.AssetController;
import com.dasi.ppt.agent.interfaces.controller.file.FileController;
import com.dasi.ppt.agent.interfaces.controller.setting.SettingController;
import jakarta.servlet.http.HttpServletRequest;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.ServletRequestBindingException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@Slf4j
@RestControllerAdvice(assignableTypes = {AssetController.class, FileController.class, SettingController.class})
public class GlobalExceptionHandler {

    @ExceptionHandler(BusinessException.class)
    public ErrorResponse businessError(BusinessException ex, HttpServletRequest request) {
        log.warn("业务请求失败：path={}，code={}，message={}", request.getRequestURI(), ex.getCode().getValue(), ex.getMessage());
        return ErrorResponse.of(ex.getCode(), ex.getMessage());
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ErrorResponse validationError(MethodArgumentNotValidException ex, HttpServletRequest request) {
        String message = ex.getBindingResult().getAllErrors().get(0).getDefaultMessage();
        log.warn("参数校验失败：path={}，code={}，message={}", request.getRequestURI(), ErrorCode.INVALID_ARGUMENT.getValue(), message);
        return ErrorResponse.of(ErrorCode.INVALID_ARGUMENT, message);
    }

    @ExceptionHandler({HttpMessageNotReadableException.class, ServletRequestBindingException.class, InvalidEnumValueException.class})
    public ErrorResponse invalidRequest(Exception ex, HttpServletRequest request) {
        log.warn("请求解析失败：path={}，code={}，message={}", request.getRequestURI(), ErrorCode.INVALID_ARGUMENT.getValue(), ex.getMessage());
        return ErrorResponse.of(ErrorCode.INVALID_ARGUMENT);
    }

    @ExceptionHandler(Exception.class)
    public ErrorResponse internalError(Exception ex, HttpServletRequest request) {
        log.error("内部错误：path={}，code={}", request.getRequestURI(), ErrorCode.INTERNAL_ERROR.getValue(), ex);
        return ErrorResponse.of(ErrorCode.INTERNAL_ERROR);
    }
}

package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolCall;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.shared.exception.LlmCallCancelledException;
import com.dasi.ppt.agent.domain.shared.exception.LlmCallException;
import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import lombok.RequiredArgsConstructor;
import okhttp3.Call;
import okhttp3.HttpUrl;
import okhttp3.Interceptor;
import okhttp3.Request;
import okhttp3.Response;
import org.springframework.ai.anthropic.AnthropicChatModel;
import org.springframework.ai.anthropic.AnthropicChatOptions;
import org.springframework.ai.chat.messages.AssistantMessage;
import org.springframework.ai.chat.messages.Message;
import org.springframework.ai.chat.messages.part.MessagePart;
import org.springframework.ai.chat.messages.part.ReasoningPart;
import org.springframework.ai.chat.metadata.EmptyUsage;
import org.springframework.ai.chat.metadata.Usage;
import org.springframework.ai.chat.model.ChatModel;
import org.springframework.ai.chat.model.ChatResponse;
import org.springframework.ai.chat.prompt.Prompt;
import org.springframework.ai.openai.responses.OpenAiResponsesChatModel;
import org.springframework.ai.openai.responses.OpenAiResponsesChatOptions;
import org.springframework.ai.tool.ToolCallback;
import org.springframework.ai.tool.definition.ToolDefinition;
import org.springframework.beans.factory.DisposableBean;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.node.ArrayNode;
import tools.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.ScheduledThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.BooleanSupplier;
import java.util.stream.Collectors;

@Component
@RequiredArgsConstructor
public class LlmGateway implements ILlmGateway, DisposableBean {

    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(80);
    private static final Set<String> THINKING_MODELS = Set.of("glm-5.3-flash", "mimo-v2.6-flash", "deepseek-flash");
    private static final Set<String> INCOMPLETE_REASONS = Set.of(
            "length", "content_filter", "incomplete", "failed", "max_tokens", "pause_turn", "refusal");
    private final ObjectProvider<OpenAiResponsesChatModel.Builder> responsesBuilders;
    private final ObjectProvider<AnthropicChatModel.Builder> anthropicBuilders;
    private final ObjectMapper objectMapper;
    private final ScheduledThreadPoolExecutor cancellationMonitor = new ScheduledThreadPoolExecutor(1);

    @Override
    public LlmResponse call(LlmRequest request, ModelConfigSettings primary, ModelConfigSettings fallback,
                            BooleanSupplier cancellationRequested) {
        List<LlmResponse.Usage> usages = new ArrayList<>();
        List<ToolCallback> tools = request.getTools().stream()
                .map(this::toolDefinition)
                .toList();
        AtomicReference<Call> activeCall = new AtomicReference<>();
        Thread caller = Thread.currentThread();
        BooleanSupplier cancelled = () -> caller.isInterrupted() || cancellationRequested.getAsBoolean();
        long deadline = System.nanoTime() + TimeUnit.MINUTES.toNanos(3);
        ScheduledFuture<?> monitor = cancellationMonitor.scheduleWithFixedDelay(() -> {
            Call active = activeCall.get();
            if (active != null && (cancelled.getAsBoolean() || System.nanoTime() >= deadline)) active.cancel();
        }, 0, 100, TimeUnit.MILLISECONDS);
        try {
            List<ModelConfigSettings> routes = fallback == null || Objects.equals(primary.getId(), fallback.getId())
                    ? List.of(primary) : List.of(primary, fallback);
            RuntimeException lastFailure = null;
            for (ModelConfigSettings model : routes) {
                for (int attempt = 0; attempt < 2; attempt++) {
                    if (cancelled.getAsBoolean()) throw new LlmCallCancelledException(usages);
                    if (System.nanoTime() >= deadline) throw new LlmCallException("模型调用超时", lastFailure, usages);
                    HttpAttempt transport = new HttpAttempt(activeCall, cancelled, usages);
                    try {
                        ChatModel client = client(model, tools, transport);
                        Prompt prompt = new Prompt(messages(request, model));
                        ChatResponse response = client.call(prompt);
                        if (transport.sent) usages.set(usages.size() - 1, usage(response.getMetadata().getUsage()));
                        if (cancelled.getAsBoolean()) throw new LlmCallCancelledException(usages);
                        return response(model, response, usages);
                    } catch (RuntimeException ex) {
                        if (cancelled.getAsBoolean() || ex instanceof LlmCallCancelledException) {
                            throw new LlmCallCancelledException(usages);
                        }
                        lastFailure = ex;
                        if (System.nanoTime() >= deadline) throw new LlmCallException("模型调用超时", ex, usages);
                        if (!transport.retryable()) break;
                    } finally {
                        activeCall.set(null);
                    }
                }
            }
            throw new LlmCallException("模型调用失败", lastFailure, usages);
        } finally {
            monitor.cancel(false);
        }
    }

    private ChatModel client(ModelConfigSettings model, List<ToolCallback> tools, Interceptor transport) {
        return switch (model.getProtocol()) {
            case RESPONSES -> responsesBuilders.getObject()
                    .options(OpenAiResponsesChatOptions.builder()
                            .baseUrl(model.getBaseUrl())
                            .apiKey(model.getKey())
                            .model(model.getModel())
                            .timeout(REQUEST_TIMEOUT)
                            .maxRetries(0)
                            .toolCallbacks(tools)
                            .build())
                    .httpClientBuilderCustomizer(builder -> builder.interceptor(transport))
                    .build();
            case ANTHROPIC -> {
                String modelName = model.getModel();
                AnthropicChatOptions.Builder options = AnthropicChatOptions.builder()
                        .baseUrl(model.getBaseUrl())
                        .apiKey(model.getKey())
                        .model(modelName)
                        .maxTokens(4096)
                        .timeout(REQUEST_TIMEOUT)
                        .maxRetries(0)
                        .toolCallbacks(tools);
                if (THINKING_MODELS.contains(modelName.substring(modelName.lastIndexOf('/') + 1))) {
                    options.thinkingEnabled(1024);
                } else {
                    options.thinkingDisabled();
                }
                // Our configured base includes the provider's API prefix. The SDK adds its own /v1.
                HttpUrl endpoint = HttpUrl.get(model.getBaseUrl()).newBuilder().addPathSegment("messages").build();
                yield anthropicBuilders.getObject()
                        .options(options.build())
                        .httpClientBuilderCustomizer(builder -> builder.interceptor(chain -> {
                            Request request = chain.request().newBuilder().url(endpoint).build();
                            return chain.proceed(request);
                        }).interceptor(transport))
                        .build();
            }
        };
    }

    private ToolCallback toolDefinition(ToolDefinition definition) {
        return new ToolCallback() {
            @Override
            public ToolDefinition getToolDefinition() {
                return definition;
            }

            @Override
            public String call(String input) {
                throw new IllegalStateException("工具由 ToolExecutor 执行");
            }
        };
    }

    private LlmResponse.Usage usage(Usage usage) {
        if (usage == null || usage instanceof EmptyUsage) {
            return LlmResponse.Usage.builder().build();
        }
        return LlmResponse.Usage.builder()
                .inputTokens(asLong(usage.getPromptTokens()))
                .outputTokens(asLong(usage.getCompletionTokens()))
                .totalTokens(asLong(usage.getTotalTokens()))
                .build();
    }

    private Long asLong(Integer tokens) {
        return tokens == null ? null : tokens.longValue();
    }

    private LlmResponse response(ModelConfigSettings model, ChatResponse response, List<LlmResponse.Usage> usages) {
        if (response.getResult() == null) throw new IllegalStateException("模型返回空响应");
        String reason = response.getResult().getMetadata().getFinishReason();
        AssistantMessage message = response.getResult().getOutput();
        if (message.getParts().isEmpty() || (reason != null && INCOMPLETE_REASONS.contains(reason.toLowerCase(Locale.ROOT)))) {
            throw new IllegalStateException("模型响应未完成：" + reason);
        }
        List<ToolCall> calls = message.getToolCalls().stream()
                .map(tool -> ToolCall.builder()
                        .toolCallId(tool.id())
                        .name(tool.name())
                        .arguments(tool.arguments())
                        .build())
                .toList();
        String reasoning = message.getReasoning().stream()
                .map(part -> part.text() == null ? part.summary() : part.text())
                .filter(Objects::nonNull)
                .collect(Collectors.joining("\n"));
        return LlmResponse.builder()
                .modelConfigId(model.getId())
                .text(message.getText())
                .reasoning(reasoning.isEmpty() ? null : reasoning)
                .toolCalls(calls)
                .usages(List.copyOf(usages))
                .raw(replayData(model, message))
                .build();
    }

    // History carries this object in the assistant message's "raw" metadata.
    private JsonNode replayData(ModelConfigSettings model, AssistantMessage message) {
        ArrayNode blocks = objectMapper.createArrayNode();
        List<MessagePart> parts = message.getParts();
        for (int index = 0; index < parts.size(); index++) {
            if (parts.get(index) instanceof ReasoningPart reasoning && reasoning.payload() != null) {
                ObjectNode block = blocks.addObject();
                block.put("index", index);
                block.set("part", objectMapper.valueToTree(reasoning));
            }
        }
        if (blocks.isEmpty()) return null;
        ObjectNode raw = objectMapper.createObjectNode();
        raw.set("source", replaySource(model));
        raw.set("blocks", blocks);
        return raw;
    }

    private JsonNode replaySource(ModelConfigSettings model) {
        return objectMapper.valueToTree(List.of(model.getProtocol(), model.getProvider(), model.getBaseUrl(), model.getModel()));
    }

    private List<Message> messages(LlmRequest request, ModelConfigSettings model) {
        List<Message> messages = new ArrayList<>(request.getMessages().size());
        for (Message message : request.getMessages()) {
            if (!(message instanceof AssistantMessage assistant) || !assistant.getMetadata().containsKey("raw")) {
                messages.add(message);
                continue;
            }
            JsonNode raw = objectMapper.valueToTree(assistant.getMetadata().get("raw"));
            List<MessagePart> parts = new ArrayList<>(assistant.getParts().stream()
                    .filter(part -> !(part instanceof ReasoningPart))
                    .toList());
            if (replaySource(model).equals(raw.path("source"))) {
                for (JsonNode block : raw.path("blocks")) {
                    ReasoningPart part = objectMapper.treeToValue(block.get("part"), ReasoningPart.class);
                    parts.add(Math.min(block.get("index").asInt(), parts.size()), part);
                }
            }
            messages.add(AssistantMessage.builder()
                    .parts(parts)
                    .properties(assistant.getMetadata())
                    .build());
        }
        return messages;
    }

    @RequiredArgsConstructor
    private static final class HttpAttempt implements Interceptor {
        private final AtomicReference<Call> activeCall;
        private final BooleanSupplier cancelled;
        private final List<LlmResponse.Usage> usages;
        private boolean sent;
        private int status;

        @Override
        public Response intercept(Chain chain) throws IOException {
            activeCall.set(chain.call());
            if (cancelled.getAsBoolean()) {
                chain.call().cancel();
                throw new IOException("模型调用已取消");
            }
            sent = true;
            usages.add(LlmResponse.Usage.builder().build());
            Response response = chain.proceed(chain.request());
            status = response.code();
            return response;
        }

        boolean retryable() {
            return sent && (status == 0 || status == 408 || status == 429 || status >= 500);
        }
    }

    @Override
    public void destroy() {
        cancellationMonitor.shutdownNow();
    }
}

```text
Messages（按序）

① System
   [按当前模式拼装的系统提示词]

② User
   <runtime_context>
     <available_skills></available_skills>
     <available_components></available_components>
     <active_skills></active_skills>
     <active_components></active_components>
     <run_state></run_state>
     <project_state></project_state>
     <current_plan></current_plan>
   </runtime_context>

③ User（有摘要时）
   <compact_summary></compact_summary>

④ 对话历史（原生消息，按原顺序）
   User：
     <conversation_history>

   User：
     [历史用户输入]

   Assistant：
     [工具调用，ID 为 call_1]

   Tool（tool_call_id = call_1）：
     [工具结果文字]
     [工具返回的图片块，如有]

   Assistant：
     [ask_user 调用，ID 为 call_2]

   Tool（tool_call_id = call_2）：
     [用户的回答]

   ……继续按历史原顺序排列……

   User：
     </conversation_history>

⑤ User
   <user_instruction>
     <user_request>
       [本轮原始请求正文]

       <!-- 可选引用按原引用顺序混排，以下仅展示条目形态 -->
       <image_attachment></image_attachment>
       [对应的图片内容块]
       <user_annotation></user_annotation>
     </user_request>

     <user_addition>
       [用户主动追加的要求]

       <!-- 同样可以携带附件、图片块和批注 -->
       <image_attachment></image_attachment>
       [对应的图片内容块]
       <user_annotation></user_annotation>
     </user_addition>

     <!-- user_addition 按接收顺序重复 -->
   </user_instruction>

⑥ User（按需）
   <runtime_instruction></runtime_instruction>


Tools（独立字段）
   [当前模式和阶段可用的工具定义]
```
